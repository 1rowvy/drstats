package store

import (
	"encoding/json"

	"github.com/1rowvy/drstats/internal/parser"
)

// SmokeVersion is the first parser version that extracts smokes; older
// matches have none and are left out of smoke stats.
const SmokeVersion = 4

// SmokeGame is what smoke stats need from one match of the account.
type SmokeGame struct {
	PlayerID   int         // the account's player
	Teams      map[int]int // player id -> team
	Smokes     []parser.Smoke
	Deaths     []parser.Death // victim, killer, assists and time only
	Objectives []parser.Objective
}

// SmokeGames returns, by match id, the smokes, hero deaths and objectives of
// the account's matches parsed with smoke data.
func (s *Store) SmokeGames(accountID uint32) (map[int64]SmokeGame, error) {
	out := map[int64]SmokeGame{}
	if accountID == 0 {
		return out, nil
	}
	rows, err := s.db.Query(`
		SELECT me.match_id, me.player_id, p.player_id, p.team
		FROM players me JOIN players p ON p.match_id = me.match_id
		WHERE me.account_id = ? AND EXISTS (
			SELECT 1 FROM replays r WHERE r.match_id = me.match_id AND r.status = 'done' AND r.parser_version >= ?)`,
		accountID, SmokeVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var me, pid, team int
		if err := rows.Scan(&id, &me, &pid, &team); err != nil {
			return nil, err
		}
		g, ok := out[id]
		if !ok {
			g = SmokeGame{PlayerID: me, Teams: map[int]int{}, Smokes: []parser.Smoke{}}
		}
		g.Teams[pid] = team
		out[id] = g
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`
		SELECT e.match_id, e.t, e.type, e.player_id, e.other_id, e.key, e.data
		FROM events e JOIN players me ON me.match_id = e.match_id AND me.account_id = ?
		WHERE e.type IN (?, ?, ?)
		ORDER BY e.match_id, e.t, e.id`, accountID, EventSmoke, EventDeath, EventObjective)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var t float64
		var typ, key, data string
		var pid, other int
		if err := rows.Scan(&id, &t, &typ, &pid, &other, &key, &data); err != nil {
			return nil, err
		}
		g, ok := out[id]
		if !ok {
			continue
		}
		switch typ {
		case EventSmoke:
			var sd SmokeData
			_ = json.Unmarshal([]byte(data), &sd)
			g.Smokes = append(g.Smokes, parser.Smoke{T: t, End: sd.End, PlayerID: pid, UserPID: other})
		case EventDeath:
			var dd DeathData
			_ = json.Unmarshal([]byte(data), &dd)
			g.Deaths = append(g.Deaths, parser.Death{T: t, VictimPID: pid, KillerPID: other, Assists: dd.Assists})
		case EventObjective:
			var od ObjectiveData
			_ = json.Unmarshal([]byte(data), &od)
			g.Objectives = append(g.Objectives, parser.Objective{T: t, Kind: od.Kind, Key: key, Team: od.Team, KillerPID: other})
		}
		out[id] = g
	}
	return out, rows.Err()
}
