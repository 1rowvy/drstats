// Package enrich fetches OpenDota extras for stored matches in the
// background, one request at a time (OpenDota's free tier is rate limited).
package enrich

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/store"
)

// EventUpdated is emitted after a match was fetched (payload: Updated).
const EventUpdated = "opendota:updated"

// Updated reports one fetch outcome.
type Updated struct {
	MatchID int64  `json:"match_id"`
	Status  string `json:"status"`
}

// RetryAfter is how long unparsed / failed matches wait before another try.
const RetryAfter = 6 * time.Hour

// Gap between requests (OpenDota allows 60/min without a key).
var Gap = 1100 * time.Millisecond

// Fetcher loads one match (opendota.Client in production).
type Fetcher interface {
	Fetch(ctx context.Context, matchID uint64) (*opendota.Match, error)
}

// Enricher owns the fetch queue.
type Enricher struct {
	st      *store.Store
	fetch   Fetcher
	emit    func(string, any)
	enabled func() bool

	mu     sync.Mutex
	queued map[int64]bool
	queue  []int64
	wake   chan struct{}
}

// New creates an enricher; enabled is checked before every request.
func New(st *store.Store, f Fetcher, emit func(string, any), enabled func() bool) *Enricher {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Enricher{st: st, fetch: f, emit: emit, enabled: enabled, queued: map[int64]bool{}, wake: make(chan struct{}, 1)}
}

// Queue schedules a match (ignored for local matches with id <= 0).
func (e *Enricher) Queue(ids ...int64) {
	e.mu.Lock()
	for _, id := range ids {
		if id > 0 && !e.queued[id] {
			e.queued[id] = true
			e.queue = append(e.queue, id)
		}
	}
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// Backfill queues every match that was never fetched or is due for a retry.
// It returns how many were queued.
func (e *Enricher) Backfill(force bool) (int, error) {
	before := time.Now().Add(-RetryAfter).Unix()
	if force {
		before = time.Now().Unix() + 1
	}
	ids, err := e.st.MatchesNeedingOpenDota(before)
	if err != nil {
		return 0, err
	}
	e.Queue(ids...)
	return len(ids), nil
}

// Run processes the queue until ctx is done.
func (e *Enricher) Run(ctx context.Context) {
	for {
		e.mu.Lock()
		var id int64
		if len(e.queue) > 0 {
			id, e.queue = e.queue[0], e.queue[1:]
		}
		e.mu.Unlock()
		if id == 0 {
			select {
			case <-ctx.Done():
				return
			case <-e.wake:
				continue
			}
		}
		if e.enabled == nil || e.enabled() {
			e.one(ctx, id)
		}
		e.mu.Lock()
		delete(e.queued, id)
		e.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(Gap):
		}
	}
}

// one fetches and stores a single match; it never panics the app.
func (e *Enricher) one(ctx context.Context, id int64) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("opendota enrich panic", "match", id, "panic", r)
		}
	}()
	m, err := e.fetch.Fetch(ctx, uint64(id))
	status, errText := opendota.StatusOK, ""
	switch {
	case errors.Is(err, opendota.ErrNotFound):
		status, m = opendota.StatusNotFound, nil
	case err != nil:
		status, errText, m = opendota.StatusError, err.Error(), nil
	case !m.Parsed:
		// Basic data only; keep it but try again later for positions/benchmarks.
		status = opendota.StatusUnparsed
	}
	if err := e.st.SaveOpenDota(id, status, errText, m); err != nil {
		slog.Error("save opendota", "match", id, "err", err)
		return
	}
	if errText != "" {
		slog.Warn("opendota fetch", "match", id, "err", errText)
	}
	e.emit(EventUpdated, Updated{MatchID: id, Status: status})
}
