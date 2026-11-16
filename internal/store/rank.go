package store

import (
	"database/sql"
	"errors"
)

// RankSeen is a medal of the account seen in its OpenDota profile.
type RankSeen struct {
	T               int64 `json:"t"` // unix
	RankTier        int   `json:"rank_tier"`
	LeaderboardRank int   `json:"leaderboard_rank"`
}

// SaveRank records the account's current medal if it differs from the last
// recorded one. A zero tier (hidden / uncalibrated) is ignored.
func (s *Store) SaveRank(accountID uint32, r RankSeen) error {
	if accountID == 0 || r.RankTier == 0 {
		return nil
	}
	var tier, lb int
	err := s.db.QueryRow(`SELECT rank_tier, leaderboard_rank FROM od_rank WHERE account_id = ? ORDER BY t DESC LIMIT 1`, accountID).
		Scan(&tier, &lb)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && tier == r.RankTier && lb == r.LeaderboardRank {
		return nil
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO od_rank (account_id, t, rank_tier, leaderboard_rank) VALUES (?, ?, ?, ?)`,
		accountID, r.T, r.RankTier, r.LeaderboardRank)
	return err
}

// Ranks returns the recorded medals of the account, oldest first.
func (s *Store) Ranks(accountID uint32) ([]RankSeen, error) {
	rows, err := s.db.Query(`SELECT t, rank_tier, leaderboard_rank FROM od_rank WHERE account_id = ? ORDER BY t`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RankSeen{}
	for rows.Next() {
		var r RankSeen
		if err := rows.Scan(&r.T, &r.RankTier, &r.LeaderboardRank); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
