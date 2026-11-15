package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/1rowvy/drstats/internal/parser"
)

// Event types stored in the events table.
const (
	EventDeath     = "death"    // player_id victim, other_id killer
	EventPurchase  = "purchase" // key item
	EventCast      = "cast"     // key ability / item_*
	EventHit       = "hit"      // key ability / item_*
	EventWard      = "ward"     // key obs|sentry, other_id killer, x/y
	EventRune      = "rune"     // key rune type
	EventStack     = "stack"
	EventBuyback   = "buyback"
	EventObjective = "objective" // key npc name, other_id killer
	EventHeroDmg   = "hdmg"      // player_id attacker, other_id target, key inflictor; 5 s bucket at t
)

// ObjectiveData is the JSON payload of an objective event.
type ObjectiveData struct {
	Kind   string `json:"kind"`
	Team   int    `json:"team"`
	Killer string `json:"killer_name,omitempty"`
}

// AmountData is the JSON payload of a hero damage bucket.
type AmountData struct {
	V int `json:"v"`
}

// DeathData is the JSON payload of a death event.
type DeathData struct {
	Killer    string `json:"killer_name"`
	Inflictor string `json:"inflictor"`
	Assists   []int  `json:"assists"`
	GoldLost  int    `json:"gold_lost,omitempty"`
	Buyback   bool   `json:"buyback,omitempty"`
}

// WardData is the JSON payload of a ward event.
type WardData struct {
	End       float64 `json:"end"`
	Team      int     `json:"team"`
	Destroyed bool    `json:"destroyed,omitempty"`
	Killer    string  `json:"killer_name,omitempty"`
}

// RuneData is the JSON payload of a rune event.
type RuneData struct {
	Bottled bool `json:"bottled,omitempty"`
}

// playerCols are the players table columns in parser.Player field order (see playerArgs).
const playerCols = `player_id, team, team_slot, hero_id, hero_name, name, account_id,
	kills, deaths, assists, level, last_hits, denies, net_worth, total_gold, total_xp, hero_damage, tower_damage, healing, rank_tier,
	gold_creep, gold_neutral, gold_hero, gold_building, gold_passive, gold_bounty, gold_roshan, gold_other,
	gold_lost, buybacks, camps_stacked, stuns, obs_placed, sen_placed, wards_destroyed, rune_pickups`

func playerArgs(p *parser.Player) []any {
	return []any{&p.PlayerID, &p.Team, &p.TeamSlot, &p.HeroID, &p.HeroName, &p.Name, &p.AccountID,
		&p.Kills, &p.Deaths, &p.Assists, &p.Level, &p.LastHits, &p.Denies, &p.NetWorth, &p.TotalGold, &p.TotalXP,
		&p.HeroDamage, &p.TowerDamage, &p.Healing, &p.RankTier,
		&p.GoldCreep, &p.GoldNeutral, &p.GoldHero, &p.GoldBuilding, &p.GoldPassive, &p.GoldBounty, &p.GoldRoshan, &p.GoldOther,
		&p.GoldLost, &p.Buybacks, &p.CampsStacked, &p.Stuns, &p.ObsPlaced, &p.SenPlaced, &p.WardsDestroyed, &p.RunePickups}
}

const seriesCols = `player_id, t, x, y, alive, level, gold, net_worth, total_gold, total_xp, last_hits, denies,
	gold_creep, gold_neutral, dmg_out, dmg_in`

func sampleArgs(r *parser.Sample) []any {
	return []any{&r.PlayerID, &r.T, &r.X, &r.Y, &r.Alive, &r.Level, &r.Gold, &r.NetWorth, &r.TotalGold, &r.TotalXP,
		&r.LastHits, &r.Denies, &r.GoldCreep, &r.GoldNeutral, &r.DmgOut, &r.DmgIn}
}

// deref turns scan destinations into insert values.
func deref(ptrs []any) []any {
	out := make([]any, len(ptrs))
	for i, p := range ptrs {
		switch v := p.(type) {
		case *int:
			out[i] = *v
		case *uint32:
			out[i] = *v
		case *string:
			out[i] = *v
		case *float64:
			out[i] = *v
		case *bool:
			out[i] = *v
		default:
			panic(fmt.Sprintf("deref: unsupported %T", p))
		}
	}
	return out
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
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
	pauses, _ := json.Marshal(m.Pauses)
	if m.Pauses == nil {
		pauses = []byte("[]")
	}
	if _, err = tx.Exec(`INSERT INTO matches (match_id, replay_id, game_build, game_mode, lobby_type, winner, duration, end_time, start_tick, pauses)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		matchID, replayID, m.GameBuild, m.GameMode, m.LobbyType, m.Winner, m.Duration, m.EndTime, m.StartTick, string(pauses)); err != nil {
		return 0, err
	}

	np := len(playerArgs(&parser.Player{}))
	ps, err := tx.Prepare(`INSERT INTO players (match_id, ` + playerCols + `) VALUES (?, ` + placeholders(np) + `)`)
	if err != nil {
		return 0, err
	}
	defer ps.Close()
	for i := range m.Players {
		if _, err = ps.Exec(append([]any{matchID}, deref(playerArgs(&m.Players[i]))...)...); err != nil {
			return 0, err
		}
	}

	ns := len(sampleArgs(&parser.Sample{}))
	ss, err := tx.Prepare(`INSERT OR REPLACE INTO series (match_id, ` + seriesCols + `) VALUES (?, ` + placeholders(ns) + `)`)
	if err != nil {
		return 0, err
	}
	defer ss.Close()
	for i := range m.Series {
		if _, err = ss.Exec(append([]any{matchID}, deref(sampleArgs(&m.Series[i]))...)...); err != nil {
			return 0, err
		}
	}

	es, err := tx.Prepare(`INSERT INTO events (match_id, t, type, player_id, other_id, x, y, key, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer es.Close()
	event := func(t float64, typ string, pid, other int, x, y any, key string, data any) error {
		d := "{}"
		if data != nil {
			b, _ := json.Marshal(data)
			d = string(b)
		}
		_, err := es.Exec(matchID, t, typ, pid, other, x, y, key, d)
		return err
	}
	for _, d := range m.Deaths {
		var x, y any
		if d.HasPos {
			x, y = d.X, d.Y
		}
		data := DeathData{Killer: d.Killer, Inflictor: d.Inflictor, Assists: d.Assists, GoldLost: d.GoldLost, Buyback: d.Buyback}
		if err = event(d.T, EventDeath, d.VictimPID, d.KillerPID, x, y, "", data); err != nil {
			return 0, err
		}
	}
	for _, p := range m.Purchases {
		if err = event(p.T, EventPurchase, p.PlayerID, -1, nil, nil, p.Item, nil); err != nil {
			return 0, err
		}
	}
	for _, c := range m.Casts {
		if err = event(c.T, EventCast, c.PlayerID, -1, nil, nil, c.Ability, nil); err != nil {
			return 0, err
		}
	}
	for _, h := range m.Hits {
		if err = event(h.T, EventHit, h.PlayerID, -1, nil, nil, h.Ability, nil); err != nil {
			return 0, err
		}
	}
	for _, w := range m.Wards {
		key := "obs"
		if w.Sentry {
			key = "sentry"
		}
		data := WardData{End: w.End, Team: w.Team, Destroyed: w.Destroyed, Killer: w.Killer}
		if err = event(w.T, EventWard, w.PlayerID, w.KillerPID, w.X, w.Y, key, data); err != nil {
			return 0, err
		}
	}
	for _, r := range m.Runes {
		var data any
		if r.Bottled {
			data = RuneData{Bottled: true}
		}
		if err = event(r.T, EventRune, r.PlayerID, -1, nil, nil, strconv.Itoa(r.Type), data); err != nil {
			return 0, err
		}
	}
	for _, mk := range m.Stacks {
		if err = event(mk.T, EventStack, mk.PlayerID, -1, nil, nil, "", nil); err != nil {
			return 0, err
		}
	}
	for _, mk := range m.Buybacks {
		if err = event(mk.T, EventBuyback, mk.PlayerID, -1, nil, nil, "", nil); err != nil {
			return 0, err
		}
	}

	for _, o := range m.Objectives {
		data := ObjectiveData{Kind: o.Kind, Team: o.Team, Killer: o.Killer}
		if err = event(o.T, EventObjective, -1, o.KillerPID, nil, nil, o.Key, data); err != nil {
			return 0, err
		}
	}
	for _, b := range m.HeroDamage {
		if err = event(b.T, EventHeroDmg, b.PlayerID, b.Target, nil, nil, b.Inflictor, AmountData{V: b.Amount}); err != nil {
			return 0, err
		}
	}

	cs, err := tx.Prepare(`INSERT INTO combat (match_id, player_id, kind, other, inflictor, amount, count) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer cs.Close()
	for _, c := range m.Combat {
		if _, err = cs.Exec(matchID, c.PlayerID, c.Kind, c.Other, c.Inflictor, c.Amount, c.Count); err != nil {
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
	Name     string `json:"name"`
	LastHits int    `json:"last_hits"`
	Denies   int    `json:"denies"`
	GPM      int    `json:"gpm"`
	LH10     *int   `json:"lh10"`     // last hits at 10:00, nil without a sample
	Position int    `json:"position"` // OpenDota estimated position 1..5, 0 if unknown
	// Lane is the detected laning-stage lane; filled by the caller from LaneEarly.
	Lane      string          `json:"lane"`
	LaneEarly []parser.Sample `json:"-"`
}

// ListMatches returns all replays (newest first) with match summaries and the
// line of the player with the given account id. Duplicate replay files of an
// already stored match are omitted.
func (s *Store) ListMatches(accountID uint32) ([]MatchRow, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.path, r.status, r.error, r.parser_version, r.mtime,
		       m.match_id, m.game_mode, m.lobby_type, m.duration, m.end_time, m.winner,
		       p.player_id, p.team, p.hero_id, p.hero_name, p.kills, p.deaths, p.assists, p.net_worth,
		       p.name, p.last_hits, p.denies, p.total_gold, s.last_hits, o.position
		FROM replays r
		LEFT JOIN matches m ON m.replay_id = r.id
		LEFT JOIN players p ON p.match_id = m.match_id AND p.account_id = ? AND ? != 0
		LEFT JOIN series s ON s.match_id = p.match_id AND s.player_id = p.player_id AND s.t = 600
		LEFT JOIN od_players o ON o.match_id = p.match_id AND o.hero_id = p.hero_id
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
		var hero, name sql.NullString
		var lh, dn, gold, lh10, pos sql.NullInt64
		if err := rows.Scan(&r.ReplayID, &r.Path, &r.Status, &r.Error, &pv, &r.MTime,
			&matchID, &mode, &lobby, &dur, &endTime, &winner,
			&pid, &team, &heroID, &hero, &k, &d, &a, &nw,
			&name, &lh, &dn, &gold, &lh10, &pos); err != nil {
			return nil, err
		}
		r.Stale = r.Status == StatusDone && pv < parser.Version
		r.MatchID, r.GameMode, r.LobbyType = matchID.Int64, int(mode.Int64), int(lobby.Int64)
		r.Duration, r.EndTime, r.Winner = dur.Float64, endTime.Int64, int(winner.Int64)
		if pid.Valid {
			r.Found = true
			r.PlayerID, r.Team, r.HeroID, r.HeroName = int(pid.Int64), int(team.Int64), int(heroID.Int64), hero.String
			r.Kills, r.Deaths, r.Assists, r.NetWorth = int(k.Int64), int(d.Int64), int(a.Int64), int(nw.Int64)
			r.Name, r.LastHits, r.Denies, r.Position = name.String, int(lh.Int64), int(dn.Int64), int(pos.Int64)
			if r.Duration >= 60 {
				r.GPM = int(math.Round(float64(gold.Int64) / (r.Duration / 60)))
			}
			if lh10.Valid {
				v := int(lh10.Int64)
				r.LH10 = &v
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if accountID == 0 {
		return out, nil
	}

	// Laning-stage positions for lane detection, in one pass.
	index := map[int64]int{}
	for i, r := range out {
		if r.Found {
			index[r.MatchID] = i
		}
	}
	rows, err = s.db.Query(`
		SELECT s.match_id, s.t, s.x, s.y, s.alive
		FROM series s JOIN players p ON p.match_id = s.match_id AND p.player_id = s.player_id
		WHERE p.account_id = ? AND s.t BETWEEN 60 AND 600
		ORDER BY s.match_id, s.t`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var smp parser.Sample
		if err := rows.Scan(&id, &smp.T, &smp.X, &smp.Y, &smp.Alive); err != nil {
			return nil, err
		}
		if i, ok := index[id]; ok {
			out[i].LaneEarly = append(out[i].LaneEarly, smp)
		}
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
	StartTick uint32  `json:"start_tick"`
}

// Match loads a stored match: meta and everything parsed (players, series,
// events, combat aggregates) as a parser.Match.
func (s *Store) Match(matchID int64) (*MatchMeta, *parser.Match, error) {
	var mm MatchMeta
	var pauses string
	err := s.db.QueryRow(`SELECT m.match_id, m.replay_id, r.path, m.game_build, m.game_mode, m.lobby_type, m.winner, m.duration, m.end_time,
		m.start_tick, m.pauses
		FROM matches m JOIN replays r ON r.id = m.replay_id WHERE m.match_id = ?`, matchID).
		Scan(&mm.MatchID, &mm.ReplayID, &mm.Path, &mm.GameBuild, &mm.GameMode, &mm.LobbyType, &mm.Winner, &mm.Duration, &mm.EndTime,
			&mm.StartTick, &pauses)
	if err != nil {
		return nil, nil, fmt.Errorf("match %d: %w", matchID, err)
	}
	m := &parser.Match{
		MatchID: uint64(max(0, mm.MatchID)), GameBuild: uint32(mm.GameBuild), GameMode: mm.GameMode, LobbyType: mm.LobbyType,
		Winner: mm.Winner, Duration: mm.Duration, EndTime: mm.EndTime, StartTick: mm.StartTick,
	}
	_ = json.Unmarshal([]byte(pauses), &m.Pauses)
	if m.Players, err = s.players(`match_id = ?`, matchID); err != nil {
		return nil, nil, err
	}
	if m.Series, err = s.series(matchID, -1); err != nil {
		return nil, nil, err
	}
	if err = s.events(m, matchID); err != nil {
		return nil, nil, err
	}
	rows, err := s.db.Query(`SELECT player_id, kind, other, inflictor, amount, count FROM combat WHERE match_id = ?`, matchID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c parser.CombatStat
		if err := rows.Scan(&c.PlayerID, &c.Kind, &c.Other, &c.Inflictor, &c.Amount, &c.Count); err != nil {
			return nil, nil, err
		}
		m.Combat = append(m.Combat, c)
	}
	return &mm, m, rows.Err()
}

func (s *Store) players(where string, args ...any) ([]parser.Player, error) {
	rows, err := s.db.Query(`SELECT `+playerCols+` FROM players WHERE `+where+` ORDER BY player_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []parser.Player
	for rows.Next() {
		var p parser.Player
		if err := rows.Scan(playerArgs(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// series loads the time series of a match (pid < 0: all players).
func (s *Store) series(matchID int64, pid int) ([]parser.Sample, error) {
	rows, err := s.db.Query(`SELECT `+seriesCols+` FROM series WHERE match_id = ? AND (? < 0 OR player_id = ?) ORDER BY player_id, t`,
		matchID, pid, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []parser.Sample
	for rows.Next() {
		var r parser.Sample
		if err := rows.Scan(sampleArgs(&r)...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) events(m *parser.Match, matchID int64) error {
	rows, err := s.db.Query(`SELECT t, type, player_id, other_id, x, y, key, data FROM events WHERE match_id = ? ORDER BY t, id`, matchID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t float64
		var typ, key, data string
		var pid, other int
		var x, y sql.NullFloat64
		if err := rows.Scan(&t, &typ, &pid, &other, &x, &y, &key, &data); err != nil {
			return err
		}
		switch typ {
		case EventDeath:
			var dd DeathData
			_ = json.Unmarshal([]byte(data), &dd)
			m.Deaths = append(m.Deaths, parser.Death{T: t, VictimPID: pid, KillerPID: other, Killer: dd.Killer,
				Inflictor: dd.Inflictor, Assists: dd.Assists, X: x.Float64, Y: y.Float64, HasPos: x.Valid,
				GoldLost: dd.GoldLost, Buyback: dd.Buyback})
		case EventPurchase:
			m.Purchases = append(m.Purchases, parser.Purchase{T: t, PlayerID: pid, Item: key})
		case EventCast:
			m.Casts = append(m.Casts, parser.Cast{T: t, PlayerID: pid, Ability: key})
		case EventHit:
			m.Hits = append(m.Hits, parser.Hit{T: t, PlayerID: pid, Ability: key})
		case EventWard:
			var wd WardData
			_ = json.Unmarshal([]byte(data), &wd)
			m.Wards = append(m.Wards, parser.Ward{T: t, End: wd.End, PlayerID: pid, Team: wd.Team, Sentry: key == "sentry",
				X: x.Float64, Y: y.Float64, Destroyed: wd.Destroyed, KillerPID: other, Killer: wd.Killer})
		case EventRune:
			var rd RuneData
			_ = json.Unmarshal([]byte(data), &rd)
			typ, _ := strconv.Atoi(key)
			m.Runes = append(m.Runes, parser.Rune{T: t, PlayerID: pid, Type: typ, Bottled: rd.Bottled})
		case EventStack:
			m.Stacks = append(m.Stacks, parser.Mark{T: t, PlayerID: pid})
		case EventBuyback:
			m.Buybacks = append(m.Buybacks, parser.Mark{T: t, PlayerID: pid})
		case EventObjective:
			var od ObjectiveData
			_ = json.Unmarshal([]byte(data), &od)
			m.Objectives = append(m.Objectives, parser.Objective{T: t, Kind: od.Kind, Key: key, Team: od.Team, KillerPID: other, Killer: od.Killer})
		case EventHeroDmg:
			var ad AmountData
			_ = json.Unmarshal([]byte(data), &ad)
			m.HeroDamage = append(m.HeroDamage, parser.DamageBucket{T: t, PlayerID: pid, Target: other, Inflictor: key, Amount: ad.V})
		}
	}
	return rows.Err()
}

// ItemTiming is the average first-purchase time of an item over matches.
type ItemTiming struct {
	Item string  `json:"item"`
	Avg  float64 `json:"avg"`
	Min  float64 `json:"min"` // best (earliest) first purchase
	N    int     `json:"n"`
}

// ItemTimings averages, per item, the first purchase time of the account on
// the hero over all stored matches except excludeMatch.
func (s *Store) ItemTimings(accountID uint32, heroID int, excludeMatch int64) ([]ItemTiming, error) {
	rows, err := s.db.Query(`
		SELECT key, AVG(first_t), MIN(first_t), COUNT(*) FROM (
			SELECT e.match_id, e.key, MIN(e.t) AS first_t
			FROM events e JOIN players p ON p.match_id = e.match_id AND p.player_id = e.player_id
			WHERE e.type = ? AND p.account_id = ? AND p.hero_id = ? AND e.match_id != ?
			GROUP BY e.match_id, e.key)
		GROUP BY key`, EventPurchase, accountID, heroID, excludeMatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ItemTiming
	for rows.Next() {
		var it ItemTiming
		if err := rows.Scan(&it.Item, &it.Avg, &it.Min, &it.N); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
