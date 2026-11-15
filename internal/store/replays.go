package store

import (
	"database/sql"
	"errors"
	"time"
)

// Replay parse statuses.
const (
	StatusPending = "pending"
	StatusParsing = "parsing"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// Replay is a replay file known to the app.
type Replay struct {
	ID            int64  `json:"id"`
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	MTime         int64  `json:"mtime"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	ParserVersion int    `json:"parser_version"`
	MatchID       int64  `json:"match_id"`
	AddedAt       int64  `json:"added_at"`
	ParsedAt      int64  `json:"parsed_at"`
}

// AddReplay registers a replay file. A known file whose size or mtime changed
// is reset to pending. It reports whether the file needs (re-)parsing.
func (s *Store) AddReplay(path string, size, mtime int64) (id int64, needsParse bool, err error) {
	var r Replay
	err = s.db.QueryRow(`SELECT id, size, mtime, status FROM replays WHERE path = ?`, path).
		Scan(&r.ID, &r.Size, &r.MTime, &r.Status)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := s.db.Exec(`INSERT INTO replays (path, size, mtime, added_at) VALUES (?, ?, ?, ?)`,
			path, size, mtime, time.Now().Unix())
		if err != nil {
			return 0, false, err
		}
		id, err := res.LastInsertId()
		return id, true, err
	case err != nil:
		return 0, false, err
	}
	if r.Size != size || r.MTime != mtime {
		_, err = s.db.Exec(`UPDATE replays SET size = ?, mtime = ?, status = 'pending', error = '' WHERE id = ?`,
			size, mtime, r.ID)
		return r.ID, true, err
	}
	return r.ID, r.Status == StatusPending, nil
}

// Replay returns one replay by id.
func (s *Store) Replay(id int64) (*Replay, error) {
	var r Replay
	var matchID, parsedAt sql.NullInt64
	err := s.db.QueryRow(`SELECT id, path, size, mtime, status, error, parser_version, match_id, added_at, parsed_at
		FROM replays WHERE id = ?`, id).
		Scan(&r.ID, &r.Path, &r.Size, &r.MTime, &r.Status, &r.Error, &r.ParserVersion, &matchID, &r.AddedAt, &parsedAt)
	if err != nil {
		return nil, err
	}
	r.MatchID, r.ParsedAt = matchID.Int64, parsedAt.Int64
	return &r, nil
}

// SetStatus updates a replay's parse status and error text.
func (s *Store) SetStatus(id int64, status, errText string) error {
	_, err := s.db.Exec(`UPDATE replays SET status = ?, error = ?, parsed_at = CASE WHEN ? IN ('done','failed') THEN ? ELSE parsed_at END WHERE id = ?`,
		status, errText, status, time.Now().Unix(), id)
	return err
}

// ResetInterrupted puts replays left in "parsing" (app closed mid-parse) back to pending.
func (s *Store) ResetInterrupted() error {
	_, err := s.db.Exec(`UPDATE replays SET status = 'pending' WHERE status = 'parsing'`)
	return err
}

// PendingReplays returns ids of replays waiting to be parsed, newest file first.
func (s *Store) PendingReplays() ([]int64, error) {
	rows, err := s.db.Query(`SELECT id FROM replays WHERE status = 'pending' ORDER BY mtime DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// StatusCounts returns the number of replays per status.
func (s *Store) StatusCounts() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM replays GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{StatusPending: 0, StatusParsing: 0, StatusDone: 0, StatusFailed: 0}
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

// SetParseTime records how long the last successful parse of a replay took.
func (s *Store) SetParseTime(id int64, ms int64) error {
	_, err := s.db.Exec(`UPDATE replays SET parse_ms = ? WHERE id = ?`, ms, id)
	return err
}

// AvgParseMS is the mean parse time over replays parsed by the given parser
// version (0 when none has been timed yet).
func (s *Store) AvgParseMS(version int) (float64, error) {
	var v float64
	err := s.db.QueryRow(`SELECT COALESCE(AVG(parse_ms), 0) FROM replays WHERE status = 'done' AND parse_ms > 0 AND parser_version = ?`, version).Scan(&v)
	return v, err
}

// DeleteReplay forgets a replay (and its match). The file itself is untouched.
func (s *Store) DeleteReplay(id int64) error {
	_, err := s.db.Exec(`DELETE FROM replays WHERE id = ?`, id)
	return err
}

// QueueReparse resets parsed and failed replays to pending and returns their
// ids. With onlyStale, only replays parsed by an older parser version (and
// failed ones, which a newer parser may handle) are reset.
func (s *Store) QueueReparse(onlyStale bool, version int) ([]int64, error) {
	rows, err := s.db.Query(`UPDATE replays SET status = 'pending', error = ''
		WHERE status IN ('done', 'failed') AND (? = 0 OR parser_version < ? OR status = 'failed')
		RETURNING id`, onlyStale, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// StaleCount returns how many parsed replays were parsed by an older parser version.
func (s *Store) StaleCount(version int) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM replays WHERE status = 'done' AND parser_version < ?`, version).Scan(&n)
	return n, err
}
