package store

import "encoding/json"

// LeadPoint is the team net worth lead at a minute, from the account's side.
type LeadPoint struct {
	T    float64
	Lead int
}

// FlowDeath is a hero death seen from the account's side.
type FlowDeath struct {
	T      float64
	Mine   bool // the account died
	Ally   bool // the victim is on the account's team
	Credit bool // the account got the kill or an assist
}

// FlowSample is the account's cumulative values at a moment.
type FlowSample struct {
	T         float64
	TotalGold int
	TotalXP   int
	LastHits  int
}

// GameFlow is what the game-flow stats need of one match.
type GameFlow struct {
	Lead    []LeadPoint  // every minute from 0:00
	Deaths  []FlowDeath  // all hero deaths, by time
	Samples []FlowSample // the account's samples every minute
}

// GameFlows returns the flow of every parsed match the account played.
func (s *Store) GameFlows(accountID uint32) (map[int64]*GameFlow, error) {
	out := map[int64]*GameFlow{}
	if accountID == 0 {
		return out, nil
	}
	get := func(id int64) *GameFlow {
		f := out[id]
		if f == nil {
			f = &GameFlow{}
			out[id] = f
		}
		return f
	}
	rows, err := s.db.Query(`
		SELECT s.match_id, s.t,
		       SUM(CASE WHEN p.team = me.team THEN s.net_worth ELSE -s.net_worth END),
		       COUNT(DISTINCT p.team)
		FROM players me
		JOIN players p ON p.match_id = me.match_id AND p.team IN (2, 3)
		JOIN series s ON s.match_id = p.match_id AND s.player_id = p.player_id
		     AND s.t >= 0 AND CAST(s.t AS INTEGER) % 60 = 0 AND s.t = CAST(s.t AS INTEGER)
		WHERE me.account_id = ?
		GROUP BY s.match_id, s.t
		ORDER BY s.match_id, s.t`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var lp LeadPoint
		var teams int
		if err := rows.Scan(&id, &lp.T, &lp.Lead, &teams); err != nil {
			return nil, err
		}
		if teams == 2 {
			get(id).Lead = append(get(id).Lead, lp)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`
		SELECT s.match_id, s.t, s.total_gold, s.total_xp, s.last_hits
		FROM players me
		JOIN series s ON s.match_id = me.match_id AND s.player_id = me.player_id
		     AND s.t >= 0 AND CAST(s.t AS INTEGER) % 60 = 0 AND s.t = CAST(s.t AS INTEGER)
		WHERE me.account_id = ?
		ORDER BY s.match_id, s.t`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var fs FlowSample
		if err := rows.Scan(&id, &fs.T, &fs.TotalGold, &fs.TotalXP, &fs.LastHits); err != nil {
			return nil, err
		}
		get(id).Samples = append(get(id).Samples, fs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`
		SELECT e.match_id, e.t, e.player_id, e.other_id, e.data, me.player_id, v.team = me.team
		FROM players me
		JOIN events e ON e.match_id = me.match_id AND e.type = ?
		JOIN players v ON v.match_id = e.match_id AND v.player_id = e.player_id
		WHERE me.account_id = ?
		ORDER BY e.match_id, e.t`, EventDeath, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var t float64
		var victim, killer, mePID int
		var data string
		var ally bool
		if err := rows.Scan(&id, &t, &victim, &killer, &data, &mePID, &ally); err != nil {
			return nil, err
		}
		fd := FlowDeath{T: t, Mine: victim == mePID, Ally: ally, Credit: killer == mePID}
		if !fd.Credit && !ally {
			var dd DeathData
			_ = json.Unmarshal([]byte(data), &dd)
			for _, a := range dd.Assists {
				if a == mePID {
					fd.Credit = true
				}
			}
		}
		get(id).Deaths = append(get(id).Deaths, fd)
	}
	return out, rows.Err()
}
