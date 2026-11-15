package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/1rowvy/drstats/internal/opendota"
)

// HistorySync is the state of the OpenDota history of one account.
type HistorySync struct {
	SyncedAt int64  `json:"synced_at"` // last attempt, unix; 0 = never
	OKAt     int64  `json:"ok_at"`     // last successful sync, unix
	Status   string `json:"status"`    // "" (never), ok, error
	Error    string `json:"error"`
	Count    int    `json:"count"` // stored matches
}

// SaveHistory upserts history matches of an account and records a successful sync.
func (s *Store) SaveHistory(accountID uint32, ms []opendota.HistoryMatch) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO od_history (account_id, match_id, start_time, duration, game_mode, lobby_type,
		radiant_win, player_slot, hero_id, kills, deaths, assists, last_hits, denies, gpm, xpm, hero_damage, tower_damage,
		hero_healing, lane, lane_role, is_roaming, party_size, leaver_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, m := range ms {
		if _, err := stmt.Exec(accountID, m.MatchID, m.StartTime, m.Duration, m.GameMode, m.LobbyType, m.RadiantWin, m.PlayerSlot,
			m.HeroID, m.Kills, m.Deaths, m.Assists, m.LastHits, m.Denies, m.GoldPerMin, m.XPPerMin, m.HeroDamage, m.TowerDamage,
			m.HeroHealing, m.Lane, m.LaneRole, m.IsRoaming, m.PartySize, m.LeaverStatus); err != nil {
			return err
		}
	}
	now := time.Now().Unix()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO od_history_sync (account_id, synced_at, ok_at, status, error) VALUES (?, ?, ?, 'ok', '')`,
		accountID, now, now); err != nil {
		return err
	}
	return tx.Commit()
}

// HistorySyncFailed records a failed sync attempt (the last success is kept).
func (s *Store) HistorySyncFailed(accountID uint32, errText string) error {
	_, err := s.db.Exec(`INSERT INTO od_history_sync (account_id, synced_at, status, error) VALUES (?, ?, 'error', ?)
		ON CONFLICT (account_id) DO UPDATE SET synced_at = excluded.synced_at, status = 'error', error = excluded.error`,
		accountID, time.Now().Unix(), errText)
	return err
}

// HistoryState returns the sync state of an account (zero value if never synced).
func (s *Store) HistoryState(accountID uint32) (HistorySync, error) {
	var h HistorySync
	err := s.db.QueryRow(`SELECT synced_at, ok_at, status, error FROM od_history_sync WHERE account_id = ?`, accountID).
		Scan(&h.SyncedAt, &h.OKAt, &h.Status, &h.Error)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return h, err
	}
	err = s.db.QueryRow(`SELECT COUNT(*) FROM od_history WHERE account_id = ?`, accountID).Scan(&h.Count)
	return h, err
}

// History returns the account's OpenDota history, oldest first.
func (s *Store) History(accountID uint32) ([]opendota.HistoryMatch, error) {
	rows, err := s.db.Query(`SELECT match_id, start_time, duration, game_mode, lobby_type, radiant_win, player_slot, hero_id,
		kills, deaths, assists, last_hits, denies, gpm, xpm, hero_damage, tower_damage, hero_healing, lane, lane_role,
		is_roaming, party_size, leaver_status
		FROM od_history WHERE account_id = ? ORDER BY start_time, match_id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []opendota.HistoryMatch{}
	for rows.Next() {
		var m opendota.HistoryMatch
		if err := rows.Scan(&m.MatchID, &m.StartTime, &m.Duration, &m.GameMode, &m.LobbyType, &m.RadiantWin, &m.PlayerSlot,
			&m.HeroID, &m.Kills, &m.Deaths, &m.Assists, &m.LastHits, &m.Denies, &m.GoldPerMin, &m.XPPerMin, &m.HeroDamage,
			&m.TowerDamage, &m.HeroHealing, &m.Lane, &m.LaneRole, &m.IsRoaming, &m.PartySize, &m.LeaverStatus); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
