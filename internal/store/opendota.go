package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/1rowvy/drstats/internal/opendota"
)

// SaveOpenDota stores a fetch outcome; m is nil unless the fetch succeeded.
func (s *Store) SaveOpenDota(matchID int64, status, errText string, m *opendota.Match) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO od_matches (match_id, status, error, fetched_at) VALUES (?, ?, ?, ?)`,
		matchID, status, errText, time.Now().Unix()); err != nil {
		return err
	}
	if m != nil {
		if _, err := tx.Exec(`DELETE FROM od_players WHERE match_id = ?`, matchID); err != nil {
			return err
		}
		for _, p := range m.Players {
			b, _ := json.Marshal(p.Benchmarks)
			if _, err := tx.Exec(`INSERT OR REPLACE INTO od_players (match_id, hero_id, position, lane_role, rank_tier, party_size, benchmarks)
				VALUES (?, ?, ?, ?, ?, ?, ?)`, matchID, p.HeroID, p.Position, p.LaneRole, p.RankTier, p.PartySize, string(b)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// OpenDotaPlayers returns the stored extras of a match by hero id (empty if none).
func (s *Store) OpenDotaPlayers(matchID int64) (map[int]opendota.Player, error) {
	rows, err := s.db.Query(`SELECT hero_id, position, lane_role, rank_tier, party_size, benchmarks FROM od_players WHERE match_id = ?`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]opendota.Player{}
	for rows.Next() {
		var p opendota.Player
		var bench string
		if err := rows.Scan(&p.HeroID, &p.Position, &p.LaneRole, &p.RankTier, &p.PartySize, &bench); err != nil {
			return nil, err
		}
		p.Benchmarks = map[string]opendota.Bench{}
		_ = json.Unmarshal([]byte(bench), &p.Benchmarks)
		out[p.HeroID] = p
	}
	return out, rows.Err()
}

// OpenDotaStatus returns the last fetch status of a match ("" if never fetched).
func (s *Store) OpenDotaStatus(matchID int64) (status string, fetchedAt int64, err error) {
	err = s.db.QueryRow(`SELECT status, fetched_at FROM od_matches WHERE match_id = ?`, matchID).Scan(&status, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	return status, fetchedAt, err
}

// MatchesNeedingOpenDota lists real (positive id) stored matches that were never
// fetched, or whose last attempt was unparsed/error before retryBefore (unix).
func (s *Store) MatchesNeedingOpenDota(retryBefore int64) ([]int64, error) {
	rows, err := s.db.Query(`
		SELECT m.match_id FROM matches m
		LEFT JOIN od_matches o ON o.match_id = m.match_id
		WHERE m.match_id > 0 AND (o.match_id IS NULL OR (o.status IN ('unparsed', 'error') AND o.fetched_at < ?))
		ORDER BY m.end_time DESC`, retryBefore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
