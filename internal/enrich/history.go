package enrich

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/store"
)

// EventHistory is emitted after a history sync attempt (payload: HistoryUpdated).
const EventHistory = "opendota:history"

// HistoryUpdated reports a finished history sync.
type HistoryUpdated struct {
	AccountID uint32            `json:"account_id"`
	State     store.HistorySync `json:"state"`
	Added     int               `json:"added"` // matches received in this sync
}

// HistoryMaxAge is how old the last successful sync may be before Sync
// (without force) fetches again.
const HistoryMaxAge = time.Hour

// HistoryTimeout bounds one history request (a full history is a few MB).
var HistoryTimeout = 3 * time.Minute

// HistoryFetcher loads a player's match history (opendota.Client in production).
type HistoryFetcher interface {
	FetchHistory(ctx context.Context, accountID uint32, days int) ([]opendota.HistoryMatch, error)
}

// History keeps the OpenDota match history of the account in the store.
type History struct {
	st      *store.Store
	fetch   HistoryFetcher
	emit    func(string, any)
	enabled func() bool
	mu      sync.Mutex // one sync at a time
}

// NewHistory creates a syncer; enabled is checked before every request.
func NewHistory(st *store.Store, f HistoryFetcher, emit func(string, any), enabled func() bool) *History {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &History{st: st, fetch: f, emit: emit, enabled: enabled}
}

// Sync fetches new matches of the account: the full history the first time,
// then only the days since the last success. Without force it does nothing
// while the last success is younger than HistoryMaxAge. It never panics.
func (h *History) Sync(ctx context.Context, accountID uint32, force bool) (err error) {
	if accountID == 0 || (h.enabled != nil && !h.enabled()) {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("opendota history panic", "panic", r)
			err = fmt.Errorf("OpenDota history: %v", r)
		}
	}()
	state, err := h.st.HistoryState(accountID)
	if err != nil {
		return err
	}
	now := time.Now()
	if !force && state.Status == "ok" && now.Sub(time.Unix(state.OKAt, 0)) < HistoryMaxAge {
		return nil
	}
	days := 0
	if state.OKAt > 0 {
		// Whole days since the last success plus a margin: OpenDota may add
		// a match a while after it ended.
		days = int(now.Sub(time.Unix(state.OKAt, 0)).Hours()/24) + 2
	}
	rctx, cancel := context.WithTimeout(ctx, HistoryTimeout)
	defer cancel()
	ms, ferr := h.fetch.FetchHistory(rctx, accountID, days)
	if ferr != nil {
		if err := h.st.HistorySyncFailed(accountID, ferr.Error()); err != nil {
			return err
		}
		slog.Warn("opendota history", "account", accountID, "err", ferr)
	} else if err := h.st.SaveHistory(accountID, ms); err != nil {
		return err
	}
	state, err = h.st.HistoryState(accountID)
	if err != nil {
		return err
	}
	h.emit(EventHistory, HistoryUpdated{AccountID: accountID, State: state, Added: len(ms)})
	return ferr
}

// Loop syncs on start and then every HistoryMaxAge until ctx is done.
// account is read on every round so a changed Steam ID is picked up.
func (h *History) Loop(ctx context.Context, account func() uint32) {
	for {
		_ = h.Sync(ctx, account(), false)
		select {
		case <-ctx.Done():
			return
		case <-time.After(HistoryMaxAge):
		}
	}
}
