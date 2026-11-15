// Package autodl downloads the replays of recent games that are in the
// account's OpenDota history but have no replay yet, and keeps the folder of
// auto-downloaded replays under a storage cap.
//
// Only files inside its own folder are ever removed: the Dota replay folder
// and manual downloads are never touched.
package autodl

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/1rowvy/drstats/internal/download"
	"github.com/1rowvy/drstats/internal/settings"
	"github.com/1rowvy/drstats/internal/store"
)

// EventState is emitted when the downloader state changes (payload: State).
const EventState = "autodl:state"

var (
	// Window is how far back games are considered: Valve keeps replays for
	// about two weeks.
	Window = 15 * 24 * time.Hour
	// RetryAfter is the wait before a failed / not-yet-available match is tried again.
	RetryAfter = time.Hour
	// Interval between rounds when nothing wakes the downloader.
	Interval = 30 * time.Minute
	// Gap between two downloads (OpenDota rate limit, Valve servers).
	Gap = 2 * time.Second
)

// Fetcher downloads replays (download.Client in production).
type Fetcher interface {
	Fetch(ctx context.Context, matchID uint64, dir string) (string, error)
	RequestParse(ctx context.Context, matchID uint64) error
}

// Policy is the storage rule for the folder.
type Policy struct {
	Keep     string // settings.KeepCap / KeepDelete / KeepAll
	CapBytes int64
}

// State is what the UI shows.
type State struct {
	Enabled     bool   `json:"enabled"`
	Current     int64  `json:"current"` // match being downloaded, 0 = idle
	Queued      int    `json:"queued"`  // left in this round
	Done        int    `json:"done"`    // downloaded, all time
	Waiting     int    `json:"waiting"` // OpenDota has no replay info yet / errors: will retry
	Expired     int    `json:"expired"` // Valve already deleted the replay
	LastError   string `json:"last_error"`
	StoredBytes int64  `json:"stored_bytes"` // kept auto-downloaded replays
	StoredFiles int    `json:"stored_files"`
	Dir         string `json:"dir"`
}

// Downloader owns the auto-download loop.
type Downloader struct {
	st      *store.Store
	fetch   Fetcher
	dir     string
	add     func(path string) error // import a downloaded file into the library
	emit    func(string, any)
	enabled func() bool
	account func() uint32
	policy  func() Policy

	wake    chan struct{}
	mu      sync.Mutex // guards current, queued, lastErr
	current int64
	queued  int
	lastErr string
	pruneMu sync.Mutex
}

// New creates a downloader saving into dir.
func New(st *store.Store, f Fetcher, dir string, add func(string) error, emit func(string, any),
	enabled func() bool, account func() uint32, policy func() Policy) *Downloader {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Downloader{st: st, fetch: f, dir: dir, add: add, emit: emit, enabled: enabled, account: account,
		policy: policy, wake: make(chan struct{}, 1)}
}

// Dir is the folder auto-downloaded replays are saved into.
func (d *Downloader) Dir() string { return d.dir }

// Wake starts a round soon (e.g. after a history sync or a settings change).
func (d *Downloader) Wake() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Run processes rounds until ctx is done.
func (d *Downloader) Run(ctx context.Context) {
	for {
		d.round(ctx)
		select {
		case <-ctx.Done():
			return
		case <-d.wake:
		case <-time.After(Interval):
		}
	}
}

func (d *Downloader) round(ctx context.Context) {
	if !d.enabled() {
		return
	}
	acc := d.account()
	if acc == 0 {
		return
	}
	now := time.Now()
	ids, err := d.st.MissingReplays(acc, now.Add(-Window).Unix(), now.Add(-RetryAfter).Unix())
	if err != nil {
		slog.Error("auto-download: missing replays", "err", err)
		return
	}
	d.set(func() { d.queued = len(ids) })
	for i, id := range ids {
		if ctx.Err() != nil || !d.enabled() {
			break
		}
		d.set(func() { d.current, d.queued = id, len(ids)-i-1 })
		d.one(ctx, id)
		select {
		case <-ctx.Done():
		case <-time.After(Gap):
		}
	}
	d.set(func() { d.current, d.queued = 0, 0 })
}

// one downloads a single match and records the outcome; it never panics.
func (d *Downloader) one(ctx context.Context, id int64) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("auto-download panic", "match", id, "panic", r)
		}
	}()
	path, err := d.fetch.Fetch(ctx, uint64(id), d.dir)
	status, errText := store.AutoOK, ""
	switch {
	case err == nil:
		if err := d.add(path); err != nil {
			status, errText = store.AutoError, "import: "+err.Error()
		}
	case errors.Is(err, download.ErrExpired):
		status = store.AutoExpired
	case errors.Is(err, download.ErrNotFound):
		status = store.AutoNotFound
	case errors.Is(err, download.ErrNoSalt):
		status = store.AutoNoSalt
		if err := d.fetch.RequestParse(ctx, uint64(id)); err != nil {
			slog.Warn("auto-download: request parse", "match", id, "err", err)
		}
	default:
		status, errText = store.AutoError, err.Error()
	}
	if ctx.Err() != nil && status == store.AutoError {
		return // app is closing: not a real failure
	}
	if err := d.st.SetAutoDownload(id, status, errText); err != nil {
		slog.Error("auto-download: save outcome", "match", id, "err", err)
	}
	if errText != "" {
		slog.Warn("auto-download", "match", id, "err", errText)
		d.set(func() { d.lastErr = errText })
	}
}

func (d *Downloader) set(f func()) {
	d.mu.Lock()
	f()
	d.mu.Unlock()
	d.emitState()
}

func (d *Downloader) emitState() {
	if s, err := d.State(); err == nil {
		d.emit(EventState, s)
	}
}

// State returns the current state with counts from the store.
func (d *Downloader) State() (State, error) {
	d.mu.Lock()
	s := State{Enabled: d.enabled(), Current: d.current, Queued: d.queued, LastError: d.lastErr, Dir: d.dir}
	d.mu.Unlock()
	counts, err := d.st.AutoDownloadCounts()
	if err != nil {
		return s, err
	}
	s.Done = counts[store.AutoOK]
	s.Waiting = counts[store.AutoNoSalt] + counts[store.AutoError]
	s.Expired = counts[store.AutoExpired] + counts[store.AutoNotFound]
	kept, err := d.st.ReplaysIn(d.dir)
	if err != nil {
		return s, err
	}
	for _, r := range kept {
		s.StoredBytes += r.Size
	}
	s.StoredFiles = len(kept)
	return s, nil
}

// Prune applies the storage policy to the folder: parsed replays are
// removed, oldest match first, until the kept ones fit (KeepCap) or all of
// them (KeepDelete). Unparsed replays are never removed. It returns how many
// files were removed.
func (d *Downloader) Prune() (int, error) {
	d.pruneMu.Lock()
	defer d.pruneMu.Unlock()
	p := d.policy()
	if p.Keep == settings.KeepAll {
		return 0, nil
	}
	kept, err := d.st.ReplaysIn(d.dir)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, r := range kept {
		total += r.Size
	}
	removed := 0
	for _, r := range kept {
		if p.Keep == settings.KeepCap && total <= p.CapBytes {
			break
		}
		if r.Status != store.StatusDone {
			continue
		}
		if err := os.Remove(r.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("auto-download: remove replay", "path", r.Path, "err", err)
			continue
		}
		if err := d.st.MarkPruned(r.ID); err != nil {
			return removed, err
		}
		total -= r.Size
		removed++
	}
	if removed > 0 {
		slog.Info("auto-download: replays removed by the storage cap", "removed", removed, "kept_bytes", total)
		d.emitState()
	}
	return removed, nil
}
