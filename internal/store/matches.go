package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/1rowvy/drstats/internal/parser"
)

// Event types stored in the events table.
const (
	EventDeath = "death"
)

// DeathData is the JSON payload of a death event.
type DeathData struct {
	Killer    string `json:"killer_name"`
	Inflictor string `json:"inflictor"`
	Assists   []int  `json:"assists"`
}

// MatchKey returns the primary key used for a parsed match: its Dota match id,
// or -replayID for replays without one (local lobbies).
func MatchKey(m *parser.Match, replayID int64) int64 {
	if m.MatchID != 0 {
		return int64(m.MatchID)
	}
	return -replayID
}

// SaveMatch stores a parsed match for a replay in one transaction, replacing
// whatever was stored for the same match id before, and marks the replay done.
func (s *Store) SaveMatch(replayID int64, m *parser.Match) (matchID int64, err error) {
	matchID = MatchKey(m, replayID)
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Another replay file of the same match may already own it: the newest parse wins.
	if _, err = tx.Exec(`DELETE FROM matches WHERE match_id = ? OR replay_id = ?`, matchID, replayID); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`INSERT INTO matches (match_id, replay_id, game_build, game_mode, lobby_type, winner, duration, end_time, start_tick)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		matchID, replayID, m.GameBuild, m.GameMode, m.LobbyType, m.Winner, m.Duration, m.EndTime, m.StartTick); err != nil {
		return 0, err
	}

	ps, err := tx.Prepare(`INSERT INTO players (match_id, player_id, team, team_slot, hero_id, hero_name, name, account_id,
		kills, deaths, assists, level, last_hits, denies, net_worth, total_gold, total_xp, hero_damage, tower_damage, healing, rank_tier)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ps.Close()
	for _, p := range m.Players {
		if _, err = ps.Exec(matchID, p.PlayerID, p.Team, p.TeamSlot, p.HeroID, p.HeroName, p.Name, p.AccountID,
			p.Kills, p.Deaths, p.Assists, p.Level, p.LastHits, p.Denies, p.NetWorth, p.TotalGold, p.TotalXP,
			p.HeroDamage, p.TowerDamage, p.Healing, p.RankTier); err != nil {
			return 0, err
		}
	}

	ss, err := tx.Prepare(`INSERT OR REPLACE INTO series (match_id, player_id, t, x, y, alive, level, gold, net_worth, total_gold, total_xp, last_hits, denies)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ss.Close()
	for _, r := range m.Series {
		if _, err = ss.Exec(matchID, r.PlayerID, r.T, r.X, r.Y, r.Alive, r.Level, r.Gold, r.NetWorth, r.TotalGold, r.TotalXP, r.LastHits, r.Denies); err != nil {
			return 0, err
		}
	}

	es, err := tx.Prepare(`INSERT INTO events (match_id, t, type, player_id, other_id, x, y, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer es.Close()
	for _, d := range m.Deaths {
		data, _ := json.Marshal(DeathData{Killer: d.Killer, Inflictor: d.Inflictor, Assists: d.Assists})
		var x, y any
		if d.HasPos {
			x, y = d.X, d.Y
		}
		if _, err = es.Exec(matchID, d.T, EventDeath, d.VictimPID, d.KillerPID, x, y, string(data)); err != nil {
			return 0, err
		}
	}

	if _, err = tx.Exec(`UPDATE replays SET status = 'done', error = '', parser_version = ?, match_id = ?, parsed_at = strftime('%s','now') WHERE id = ?`,
		parser.Version, matchID, replayID); err != nil {
		return 0, err
	}
	return matchID, tx.Commit()
}

// MatchRow is one line of the match list: a replay and, when parsed, its
// match summary and the focused player's line.
type MatchRow struct {
	ReplayID  int64   `json:"replay_id"`
	Path      string  `json:"path"`
	Status    string  `json:"status"`
	Error     string  `json:"error"`
	Stale     bool    `json:"stale"` // parsed by an older parser version
	MTime     int64   `json:"mtime"`
	MatchID   int64   `json:"match_id"`
	GameMode  int     `json:"game_mode"`
	LobbyType int     `json:"lobby_type"`
	Duration  float64 `json:"duration"`
	EndTime   int64   `json:"end_time"`
	Winner    int     `json:"winner"`
	// Focused player ("me"); Found is false when the account is not in the match.
	Found    bool   `json:"found"`
	PlayerID int    `json:"player_id"`
	Team     int    `json:"team"`
	HeroID   int    `json:"hero_id"`
	HeroName string `json:"hero_name"`
	Kills    int    `json:"kills"`
	Deaths   int    `json:"deaths"`
	Assists  int    `json:"assists"`
	NetWorth int    `json:"net_worth"`
}

// ListMatches returns all replays (newest first) with match summaries and the
// line of the player with the given account id. Duplicate replay files of an
// already stored match are omitted.
func (s *Store) ListMatches(accountID uint32) ([]MatchRow, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.path, r.status, r.error, r.parser_version, r.mtime,
		       m.match_id, m.game_mode, m.lobby_type, m.duration, m.end_time, m.winner,
		       p.player_id, p.team, p.hero_id, p.hero_name, p.kills, p.deaths, p.assists, p.net_worth
		FROM replays r
		LEFT JOIN matches m ON m.replay_id = r.id
		LEFT JOIN players p ON p.match_id = m.match_id AND p.account_id = ? AND ? != 0
		WHERE m.match_id IS NOT NULL OR r.status != 'done'
		ORDER BY COALESCE(m.end_time, r.mtime) DESC`, accountID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchRow
	for rows.Next() {
		var r MatchRow
		var pv int
		var matchID, mode, lobby, endTime, winner sql.NullInt64
		var dur sql.NullFloat64
		var pid, team, heroID, k, d, a, nw sql.NullInt64
		var hero sql.NullString
		if err := rows.Scan(&r.ReplayID, &r.Path, &r.Status, &r.Error, &pv, &r.MTime,
			&matchID, &mode, &lobby, &dur, &endTime, &winner,
			&pid, &team, &heroID, &hero, &k, &d, &a, &nw); err != nil {
			return nil, err
		}
		r.Stale = r.Status == StatusDone && pv < parser.Version
		r.MatchID, r.GameMode, r.LobbyType = matchID.Int64, int(mode.Int64), int(lobby.Int64)
		r.Duration, r.EndTime, r.Winner = dur.Float64, endTime.Int64, int(winner.Int64)
		if pid.Valid {
			r.Found = true
			r.PlayerID, r.Team, r.HeroID, r.HeroName = int(pid.Int64), int(team.Int64), int(heroID.Int64), hero.String
			r.Kills, r.Deaths, r.Assists, r.NetWorth = int(k.Int64), int(d.Int64), int(a.Int64), int(nw.Int64)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MatchMeta is the stored match summary.
type MatchMeta struct {
	MatchID   int64   `json:"match_id"`
	ReplayID  int64   `json:"replay_id"`
	Path      string  `json:"path"`
	GameBuild int     `json:"game_build"`
	GameMode  int     `json:"game_mode"`
	LobbyType int     `json:"lobby_type"`
	Winner    int     `json:"winner"`
	Duration  float64 `json:"duration"`
	EndTime   int64   `json:"end_time"`
}

// Match loads a stored match: meta, players, full series and death events.
func (s *Store) Match(matchID int64) (*MatchMeta, []parser.Player, []parser.Sample, []parser.Death, error) {
	var mm MatchMeta
	err := s.db.QueryRow(`SELECT m.match_id, m.replay_id, r.path, m.game_build, m.game_mode, m.lobby_type, m.winner, m.duration, m.end_time
		FROM matches m JOIN replays r ON r.id = m.replay_id WHERE m.match_id = ?`, matchID).
		Scan(&mm.MatchID, &mm.ReplayID, &mm.Path, &mm.GameBuild, &mm.GameMode, &mm.LobbyType, &mm.Winner, &mm.Duration, &mm.EndTime)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("match %d: %w", matchID, err)
	}

	var players []parser.Player
	rows, err := s.db.Query(`SELECT player_id, team, team_slot, hero_id, hero_name, name, account_id, kills, deaths, assists, level,
		last_hits, denies, net_worth, total_gold, total_xp, hero_damage, tower_damage, healing, rank_tier
		FROM players WHERE match_id = ? ORDER BY player_id`, matchID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for rows.Next() {
		var p parser.Player
		if err := rows.Scan(&p.PlayerID, &p.Team, &p.TeamSlot, &p.HeroID, &p.HeroName, &p.Name, &p.AccountID, &p.Kills, &p.Deaths,
			&p.Assists, &p.Level, &p.LastHits, &p.Denies, &p.NetWorth, &p.TotalGold, &p.TotalXP, &p.HeroDamage, &p.TowerDamage,
			&p.Healing, &p.RankTier); err != nil {
			rows.Close()
			return nil, nil, nil, nil, err
		}
		players = append(players, p)
	}
	rows.Close()

	var series []parser.Sample
	rows, err = s.db.Query(`SELECT player_id, t, x, y, alive, level, gold, net_worth, total_gold, total_xp, last_hits, denies
		FROM series WHERE match_id = ? ORDER BY player_id, t`, matchID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for rows.Next() {
		var r parser.Sample
		if err := rows.Scan(&r.PlayerID, &r.T, &r.X, &r.Y, &r.Alive, &r.Level, &r.Gold, &r.NetWorth, &r.TotalGold, &r.TotalXP,
			&r.LastHits, &r.Denies); err != nil {
			rows.Close()
			return nil, nil, nil, nil, err
		}
		series = append(series, r)
	}
	rows.Close()

	var deaths []parser.Death
	rows, err = s.db.Query(`SELECT t, player_id, other_id, x, y, data FROM events WHERE match_id = ? AND type = ? ORDER BY t`,
		matchID, EventDeath)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d parser.Death
		var x, y sql.NullFloat64
		var data string
		if err := rows.Scan(&d.T, &d.VictimPID, &d.KillerPID, &x, &y, &data); err != nil {
			return nil, nil, nil, nil, err
		}
		var dd DeathData
		_ = json.Unmarshal([]byte(data), &dd)
		d.Killer, d.Inflictor, d.Assists = dd.Killer, dd.Inflictor, dd.Assists
		d.X, d.Y, d.HasPos = x.Float64, y.Float64, x.Valid
		deaths = append(deaths, d)
	}
	return &mm, players, series, deaths, rows.Err()
}
