package store

import (
	"encoding/json"
	"sort"

	"github.com/1rowvy/drstats/internal/parser"
)

// DeathScene is a death of the account with what was around it: the last
// series sample of every hero at or before the death and the observer wards
// of the victim's team. Enough to judge whether the death was avoidable
// without loading the whole match.
type DeathScene struct {
	MatchID int64
	Death   parser.Death
	Team    int
	// Heroes are the last samples (≤ death time, within one series step)
	// of all heroes, the victim included; Teams maps player id to team.
	Heroes []parser.Sample
	Teams  map[int]int
	Wards  []parser.Ward // observer wards of the victim's team in that match
}

// DeathScenes returns every death of the account, by match and time.
func (s *Store) DeathScenes(accountID uint32) ([]DeathScene, error) {
	out := []DeathScene{}
	if accountID == 0 {
		return out, nil
	}
	rows, err := s.db.Query(`
		SELECT e.id, e.match_id, e.t, e.x, e.y, e.other_id, e.data, p.player_id, p.team
		FROM events e JOIN players p ON p.match_id = e.match_id AND p.player_id = e.player_id
		WHERE e.type = ? AND p.account_id = ?
		ORDER BY e.match_id, e.t`, EventDeath, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int64]int{}
	for rows.Next() {
		var id int64
		var sc DeathScene
		var x, y *float64
		var data string
		d := &sc.Death
		if err := rows.Scan(&id, &sc.MatchID, &d.T, &x, &y, &d.KillerPID, &data, &d.VictimPID, &sc.Team); err != nil {
			return nil, err
		}
		if x != nil && y != nil {
			d.X, d.Y, d.HasPos = *x, *y, true
		}
		var dd DeathData
		_ = json.Unmarshal([]byte(data), &dd)
		d.Killer, d.Inflictor, d.Assists, d.GoldLost, d.Buyback = dd.Killer, dd.Inflictor, dd.Assists, dd.GoldLost, dd.Buyback
		sc.Teams = map[int]int{}
		byID[id] = len(out)
		out = append(out, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Samples of every hero in the series step before each death.
	rows, err = s.db.Query(`
		SELECT e.id, s.player_id, q.team, s.t, s.x, s.y, s.alive, s.gold
		FROM events e
		JOIN players p ON p.match_id = e.match_id AND p.player_id = e.player_id
		JOIN series s ON s.match_id = e.match_id AND s.t <= e.t AND s.t > e.t - ?
		JOIN players q ON q.match_id = s.match_id AND q.player_id = s.player_id
		WHERE e.type = ? AND p.account_id = ?`, parser.SeriesStep, EventDeath, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var team int
		var smp parser.Sample
		if err := rows.Scan(&id, &smp.PlayerID, &team, &smp.T, &smp.X, &smp.Y, &smp.Alive, &smp.Gold); err != nil {
			return nil, err
		}
		i, ok := byID[id]
		if !ok {
			continue
		}
		out[i].Teams[smp.PlayerID] = team
		out[i].Heroes = append(out[i].Heroes, smp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range out {
		out[i].Heroes = latestPerPlayer(out[i].Heroes)
	}

	// Observer wards of the matches with a death.
	rows, err = s.db.Query(`
		SELECT w.match_id, w.t, w.player_id, w.x, w.y, w.data
		FROM events w
		WHERE w.type = ? AND w.key = 'obs' AND w.x IS NOT NULL AND w.match_id IN (
			SELECT e.match_id FROM events e JOIN players p ON p.match_id = e.match_id AND p.player_id = e.player_id
			WHERE e.type = ? AND p.account_id = ?)`, EventWard, EventDeath, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	wards := map[int64][]parser.Ward{}
	for rows.Next() {
		var mid int64
		var w parser.Ward
		var data string
		if err := rows.Scan(&mid, &w.T, &w.PlayerID, &w.X, &w.Y, &data); err != nil {
			return nil, err
		}
		var wd WardData
		_ = json.Unmarshal([]byte(data), &wd)
		w.End, w.Team, w.Destroyed = wd.End, wd.Team, wd.Destroyed
		wards[mid] = append(wards[mid], w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		for _, w := range wards[out[i].MatchID] {
			if w.Team == out[i].Team {
				out[i].Wards = append(out[i].Wards, w)
			}
		}
	}
	return out, nil
}

// latestPerPlayer keeps the last sample of each player.
func latestPerPlayer(ss []parser.Sample) []parser.Sample {
	last := map[int]parser.Sample{}
	for _, s := range ss {
		if o, ok := last[s.PlayerID]; !ok || s.T > o.T {
			last[s.PlayerID] = s
		}
	}
	out := make([]parser.Sample, 0, len(last))
	for _, s := range last {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PlayerID < out[j].PlayerID })
	return out
}
