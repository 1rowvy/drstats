package store

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Auto-download outcomes.
const (
	AutoOK       = "ok"
	AutoExpired  = "expired"  // Valve has no replay: final for old matches, retried for fresh ones
	AutoNotFound = "notfound" // OpenDota does not know the match: final
	AutoNoSalt   = "nosalt"   // OpenDota has no replay info yet: retry
	AutoError    = "error"    // network etc.: retry
)

// Retry says when failed auto-downloads are tried again (all values unix).
type Retry struct {
	Before int64 // failed attempts older than this are retried
	// Fresh matches (started after FreshSince) are retried sooner: right after
	// a game OpenDota has no salt yet and Valve answers 404 until the replay is
	// uploaded, so "expired" is not final for them either. The first
	// FastAttempts attempts are retried once older than FastBefore.
	FreshSince   int64
	FastBefore   int64
	FastAttempts int
}

// MissingReplays lists history matches of the account that started after
// since (unix), have no stored match or replay, and are not finished or
// waiting for a retry in auto_downloads. Newest first.
func (s *Store) MissingReplays(accountID uint32, since int64, r Retry) ([]int64, error) {
	rows, err := s.db.Query(`
		SELECT h.match_id FROM od_history h
		LEFT JOIN auto_downloads a ON a.match_id = h.match_id
		WHERE h.account_id = ? AND h.start_time >= ?
		  AND NOT EXISTS (SELECT 1 FROM matches m WHERE m.match_id = h.match_id)
		  AND NOT EXISTS (SELECT 1 FROM replays r WHERE r.match_id = h.match_id)
		  AND (a.match_id IS NULL
		    OR (a.status IN ('nosalt', 'error') AND a.tried_at < ?)
		    OR (a.status IN ('nosalt', 'error', 'expired') AND h.start_time >= ?
		        AND (a.tried_at < ? OR (a.attempts < ? AND a.tried_at < ?))))
		ORDER BY h.start_time DESC`, accountID, since,
		r.Before, r.FreshSince, r.Before, r.FastAttempts, r.FastBefore)
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

// SetAutoDownload records the outcome of an auto-download attempt.
func (s *Store) SetAutoDownload(matchID int64, status, errText string) error {
	_, err := s.db.Exec(`INSERT INTO auto_downloads (match_id, status, error, tried_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (match_id) DO UPDATE SET status = excluded.status, error = excluded.error,
		  tried_at = excluded.tried_at, attempts = attempts + 1`, matchID, status, errText, time.Now().Unix())
	return err
}

// AutoExpiredFresh is the AutoDownloadCounts key of "expired" matches that
// started after freshSince: they are still retried, so not really gone.
const AutoExpiredFresh = "expired_fresh"

// AutoDownloadCounts returns the number of auto-download outcomes per status.
func (s *Store) AutoDownloadCounts(freshSince int64) (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT CASE WHEN a.status = 'expired' AND EXISTS (
		         SELECT 1 FROM od_history h WHERE h.match_id = a.match_id AND h.start_time >= ?)
		       THEN 'expired_fresh' ELSE a.status END AS st, COUNT(*)
		FROM auto_downloads a GROUP BY st`, freshSince)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// StoredReplay is a kept replay file in a folder, for the storage cap.
type StoredReplay struct {
	ID     int64
	Path   string
	Size   int64
	Status string
}

// ReplaysIn lists not-yet-pruned replays whose path is inside dir, oldest
// match first (unparsed ones, without a match, by file time).
func (s *Store) ReplaysIn(dir string) ([]StoredReplay, error) {
	prefix := strings.TrimRight(dir, `/\`)
	n := utf8.RuneCountInString(prefix) // SQLite substr counts characters
	rows, err := s.db.Query(`
		SELECT r.id, r.path, r.size, r.status FROM replays r
		LEFT JOIN matches m ON m.replay_id = r.id
		WHERE r.pruned_at = 0 AND substr(r.path, 1, ?) = ? AND substr(r.path, ? + 1, 1) IN ('/', '\')
		ORDER BY COALESCE(m.end_time, r.mtime), r.id`, n, prefix, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoredReplay
	for rows.Next() {
		var r StoredReplay
		if err := rows.Scan(&r.ID, &r.Path, &r.Size, &r.Status); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkPruned records that a replay file was removed by the storage cap.
func (s *Store) MarkPruned(id int64) error {
	_, err := s.db.Exec(`UPDATE replays SET pruned_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}
