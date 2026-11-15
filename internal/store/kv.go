package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// GetSetting decodes the JSON value stored under key into v.
// It reports false (and leaves v untouched) when the key is absent.
func (s *Store) GetSetting(key string, v any) (bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), v)
}

// SetSetting stores v as JSON under key.
func (s *Store) SetSetting(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, string(b))
	return err
}

// Constant returns a cached constants blob and when it was fetched (zero time if absent).
func (s *Store) Constant(name string) ([]byte, time.Time, error) {
	var data string
	var at int64
	err := s.db.QueryRow(`SELECT data, fetched_at FROM constants WHERE name = ?`, name).Scan(&data, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	return []byte(data), time.Unix(at, 0), nil
}

// PutConstant caches a constants blob.
func (s *Store) PutConstant(name string, data []byte) error {
	_, err := s.db.Exec(`INSERT INTO constants (name, data, fetched_at) VALUES (?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET data = excluded.data, fetched_at = excluded.fetched_at`,
		name, string(data), time.Now().Unix())
	return err
}
