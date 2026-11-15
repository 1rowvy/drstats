package store

import (
	"database/sql"

	"github.com/1rowvy/drstats/internal/parser"
)

// GameRow is one stored match from the point of view of one account.
type GameRow struct {
	MatchID   int64   `json:"match_id"`
	EndTime   int64   `json:"end_time"`
	Duration  float64 `json:"duration"`
	Winner    int     `json:"winner"`
	GameMode  int     `json:"game_mode"`
	LobbyType int     `json:"lobby_type"`

	PlayerID     int     `json:"player_id"`
	Team         int     `json:"team"`
	HeroID       int     `json:"hero_id"`
	HeroName     string  `json:"hero_name"`
	Kills        int     `json:"kills"`
	Deaths       int     `json:"deaths"`
	Assists      int     `json:"assists"`
	LastHits     int     `json:"last_hits"`
	Denies       int     `json:"denies"`
	NetWorth     int     `json:"net_worth"`
	TotalGold    int     `json:"total_gold"`
	TotalXP      int     `json:"total_xp"`
	HeroDamage   int     `json:"hero_damage"`
	TowerDamage  int     `json:"tower_damage"`
	Healing      int     `json:"healing"`
	GoldLost     int     `json:"gold_lost"`
	ObsPlaced    int     `json:"obs_placed"`
	SenPlaced    int     `json:"sen_placed"`
	CampsStacked int     `json:"camps_stacked"`
	Stuns        float64 `json:"stuns"`
	TeamKills    int     `json:"team_kills"`

	// Laning stage (nil when the series has no sample at 10:00).
	LH10      *int            `json:"lh10"`
	DN10      *int            `json:"dn10"`
	NW10      *int            `json:"nw10"`
	XP10      *int            `json:"xp10"`
	Deaths10  int             `json:"deaths10"` // deaths before 10:00
	LaneEarly []parser.Sample `json:"-"`        // alive samples 1:00-10:00, for lane detection
}

// Games returns every parsed match the account played, oldest first.
func (s *Store) Games(accountID uint32) ([]GameRow, error) {
	if accountID == 0 {
		return []GameRow{}, nil
	}
	rows, err := s.db.Query(`
		SELECT m.match_id, m.end_time, m.duration, m.winner, m.game_mode, m.lobby_type,
		       p.player_id, p.team, p.hero_id, p.hero_name, p.kills, p.deaths, p.assists, p.last_hits, p.denies,
		       p.net_worth, p.total_gold, p.total_xp, p.hero_damage, p.tower_damage, p.healing, p.gold_lost,
		       p.obs_placed, p.sen_placed, p.camps_stacked, p.stuns,
		       (SELECT COALESCE(SUM(q.kills), 0) FROM players q WHERE q.match_id = m.match_id AND q.team = p.team),
		       s.last_hits, s.denies, s.net_worth, s.total_xp,
		       (SELECT COUNT(*) FROM events e WHERE e.match_id = m.match_id AND e.type = ? AND e.player_id = p.player_id AND e.t <= 600)
		FROM players p
		JOIN matches m ON m.match_id = p.match_id
		LEFT JOIN series s ON s.match_id = m.match_id AND s.player_id = p.player_id AND s.t = 600
		WHERE p.account_id = ?
		ORDER BY m.end_time, m.match_id`, EventDeath, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GameRow{}
	index := map[int64]int{}
	for rows.Next() {
		var g GameRow
		var lh, dn, nw, xp sql.NullInt64
		if err := rows.Scan(&g.MatchID, &g.EndTime, &g.Duration, &g.Winner, &g.GameMode, &g.LobbyType,
			&g.PlayerID, &g.Team, &g.HeroID, &g.HeroName, &g.Kills, &g.Deaths, &g.Assists, &g.LastHits, &g.Denies,
			&g.NetWorth, &g.TotalGold, &g.TotalXP, &g.HeroDamage, &g.TowerDamage, &g.Healing, &g.GoldLost,
			&g.ObsPlaced, &g.SenPlaced, &g.CampsStacked, &g.Stuns, &g.TeamKills,
			&lh, &dn, &nw, &xp, &g.Deaths10); err != nil {
			return nil, err
		}
		if lh.Valid {
			a, b, c, d := int(lh.Int64), int(dn.Int64), int(nw.Int64), int(xp.Int64)
			g.LH10, g.DN10, g.NW10, g.XP10 = &a, &b, &c, &d
		}
		index[g.MatchID] = len(out)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Laning-stage positions for lane detection, in one pass.
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

// DeathPoint is a death position of the account.
type DeathPoint struct {
	MatchID int64   `json:"match_id"`
	HeroID  int     `json:"hero_id"`
	T       float64 `json:"t"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Region  string  `json:"region"` // map zone key, filled by stats
}

// DeathPoints returns all positioned deaths of the account (heroID 0 = any hero).
func (s *Store) DeathPoints(accountID uint32, heroID int) ([]DeathPoint, error) {
	rows, err := s.db.Query(`
		SELECT e.match_id, p.hero_id, e.t, e.x, e.y
		FROM events e JOIN players p ON p.match_id = e.match_id AND p.player_id = e.player_id
		WHERE e.type = ? AND p.account_id = ? AND (? = 0 OR p.hero_id = ?) AND e.x IS NOT NULL`,
		EventDeath, accountID, heroID, heroID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeathPoint{}
	for rows.Next() {
		var d DeathPoint
		if err := rows.Scan(&d.MatchID, &d.HeroID, &d.T, &d.X, &d.Y); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// AbilityTotal is the cast/hit count of an ability over many matches.
type AbilityTotal struct {
	Ability string `json:"ability"`
	Casts   int    `json:"casts"`
	Matches int    `json:"matches"`
}

// AbilityCasts sums casts per ability of the account on a hero.
func (s *Store) AbilityCasts(accountID uint32, heroID int) ([]AbilityTotal, error) {
	rows, err := s.db.Query(`
		SELECT e.key, COUNT(*), COUNT(DISTINCT e.match_id)
		FROM events e JOIN players p ON p.match_id = e.match_id AND p.player_id = e.player_id
		WHERE e.type = ? AND p.account_id = ? AND p.hero_id = ?
		GROUP BY e.key ORDER BY COUNT(*) DESC`, EventCast, accountID, heroID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AbilityTotal{}
	for rows.Next() {
		var a AbilityTotal
		if err := rows.Scan(&a.Ability, &a.Casts, &a.Matches); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
