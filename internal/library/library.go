// Package library finds replay files, registers them in the store and parses
// them in a background worker pool, reporting progress through an emit hook.
package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

// Event names emitted by the library.
const (
	EventReplay   = "library:replay"   // payload: ReplayEvent
	EventProgress = "library:progress" // payload: Progress
	EventParsing  = "library:parsing"  // payload: ParseProgress
)

// ParseProgress is how far a running parse has read its replay file.
type ParseProgress struct {
	ReplayID int64   `json:"replay_id"`
	Fraction float64 `json:"fraction"` // 0..1
}

// ReplayEvent reports a status change of one replay.
type ReplayEvent struct {
	ReplayID int64  `json:"replay_id"`
	Status   string `json:"status"`
	Error    string `json:"error"`
	MatchID  int64  `json:"match_id"`
}

// Progress is the overall queue state.
type Progress struct {
	Pending int `json:"pending"`
	Parsing int `json:"parsing"`
	Done    int `json:"done"`
	Failed  int `json:"failed"`
	Stale   int `json:"stale"` // parsed by an older parser version
	// AvgParseMS is the mean parse time of replays parsed by the current parser.
	AvgParseMS float64 `json:"avg_parse_ms"`
}

// ParseFunc parses one replay file, reporting the read fraction (0..1) to
// progress; replaceable in tests.
type ParseFunc func(path string, progress func(float64)) (*parser.Match, error)

// parseFile parses a replay, measuring progress by bytes read from the file.
func parseFile(path string, progress func(float64)) (*parser.Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return parser.Parse(&countingReader{r: f, total: st.Size(), progress: progress})
}

// countingReader reports read progress in steps of at least 2%.
type countingReader struct {
	r        io.Reader
	n, total int64
	last     float64
	progress func(float64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.total > 0 && c.progress != nil {
		if f := float64(c.n) / float64(c.total); f-c.last >= 0.02 {
			c.last = f
			c.progress(min(f, 1))
		}
	}
	return n, err
}

// Library owns the parse queue.
type Library struct {
	st    *store.Store
	emit  func(name string, data any)
	parse ParseFunc

	mu      sync.Mutex
	cond    *sync.Cond
	pending []int64
	queued  map[int64]bool
	running int
	limit   int
	stopped bool
	idle    sync.WaitGroup
}

// New creates a library; call Start to begin parsing.
func New(st *store.Store, emit func(string, any)) *Library {
	if emit == nil {
		emit = func(string, any) {}
	}
	l := &Library{st: st, emit: emit, parse: parseFile, queued: map[int64]bool{}, limit: 1}
	l.cond = sync.NewCond(&l.mu)
	return l
}

// Start launches the dispatcher with the given parallelism and queues every
// replay left pending (including ones interrupted by a previous shutdown).
func (l *Library) Start(parallelism int) error {
	l.SetParallelism(parallelism)
	if err := l.st.ResetInterrupted(); err != nil {
		return err
	}
	ids, err := l.st.PendingReplays()
	if err != nil {
		return err
	}
	for _, id := range ids {
		l.enqueue(id)
	}
	go l.loop()
	l.emitProgress()
	return nil
}

// Stop prevents new parses from starting. Running ones finish in the background;
// their replays are reset to pending on the next Start if the app exits first.
func (l *Library) Stop() {
	l.mu.Lock()
	l.stopped = true
	l.cond.Broadcast()
	l.mu.Unlock()
}

// Wait blocks until the queue is empty and no parse is running (tests).
func (l *Library) Wait() {
	l.mu.Lock()
	for len(l.pending) > 0 || l.running > 0 {
		l.cond.Wait()
	}
	l.mu.Unlock()
}

// SetParallelism changes how many replays are parsed at once.
func (l *Library) SetParallelism(n int) {
	l.mu.Lock()
	l.limit = max(1, n)
	l.cond.Broadcast()
	l.mu.Unlock()
}

func (l *Library) enqueue(id int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.queued[id] {
		return
	}
	l.queued[id] = true
	l.pending = append(l.pending, id)
	l.cond.Broadcast()
}

func (l *Library) loop() {
	for {
		l.mu.Lock()
		for !l.stopped && (len(l.pending) == 0 || l.running >= l.limit) {
			l.cond.Wait()
		}
		if l.stopped {
			l.mu.Unlock()
			return
		}
		id := l.pending[0]
		l.pending = l.pending[1:]
		l.running++
		l.mu.Unlock()

		go func() {
			l.work(id)
			l.mu.Lock()
			l.running--
			delete(l.queued, id)
			l.cond.Broadcast()
			l.mu.Unlock()
			l.emitProgress()
		}()
	}
}

// work parses one replay. Any failure (including panics outside manta) marks
// the replay failed with the error text; it never takes the app down.
func (l *Library) work(id int64) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("internal panic: %v", r)
			slog.Error("replay worker panic", "replay", id, "panic", r, "stack", string(debug.Stack()))
			l.fail(id, msg)
		}
	}()
	r, err := l.st.Replay(id)
	if err != nil {
		slog.Error("load replay", "replay", id, "err", err)
		return
	}
	if err := l.st.SetStatus(id, store.StatusParsing, ""); err != nil {
		slog.Error("set status", "replay", id, "err", err)
	}
	// The file may be gone by now (a partial demo Dota deleted): forget it.
	if _, err := os.Stat(r.Path); errors.Is(err, fs.ErrNotExist) && l.forget(id) {
		l.emitProgress()
		return
	}
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusParsing})
	l.emitProgress()

	started := time.Now()
	m, err := l.parse(r.Path, func(f float64) {
		l.emit(EventParsing, ParseProgress{ReplayID: id, Fraction: f})
	})
	if err != nil {
		var pe *parser.ParseError
		if errors.As(err, &pe) && pe.Panic {
			slog.Error("parser panic", "path", r.Path, "panic", pe.Msg, "stack", pe.Stack)
		}
		l.fail(id, err.Error())
		return
	}
	matchID, err := l.st.SaveMatch(id, m)
	if err != nil {
		l.fail(id, "save: "+err.Error())
		return
	}
	if err := l.st.SetParseTime(id, time.Since(started).Milliseconds()); err != nil {
		slog.Warn("set parse time", "replay", id, "err", err)
	}
	slog.Info("replay parsed", "path", r.Path, "match", matchID, "took", time.Since(started).Round(time.Millisecond))
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusDone, MatchID: matchID})
}

func (l *Library) fail(id int64, msg string) {
	if err := l.st.SetStatus(id, store.StatusFailed, msg); err != nil {
		slog.Error("set failed status", "replay", id, "err", err)
	}
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusFailed, Error: msg})
}

func (l *Library) emitProgress() {
	l.emit(EventProgress, l.Progress())
}

// Progress returns the current queue state.
func (l *Library) Progress() Progress {
	c, _ := l.st.StatusCounts()
	stale, _ := l.st.StaleCount(parser.Version)
	avg, _ := l.st.AvgParseMS(parser.Version)
	return Progress{Pending: c[store.StatusPending], Parsing: c[store.StatusParsing],
		Done: c[store.StatusDone], Failed: c[store.StatusFailed], Stale: stale, AvgParseMS: avg}
}

// IsReplay reports whether a path looks like a Dota 2 replay file.
func IsReplay(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".dem")
}

// Import registers files and/or directories (walked recursively) and queues
// new or changed replays. It returns how many replays were queued.
func (l *Library) Import(paths []string) (int, error) {
	return l.importPaths(paths, 0)
}

// importPaths is Import that skips files modified less than minAge ago
// (a replay Dota is still writing).
func (l *Library) importPaths(paths []string, minAge time.Duration) (int, error) {
	queued := 0
	fresh := func(info fs.FileInfo) bool { return minAge > 0 && time.Since(info.ModTime()) < minAge }
	var errs []error
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !st.IsDir() {
			if !IsReplay(p) {
				errs = append(errs, fmt.Errorf("%s: not a .dem replay", filepath.Base(p)))
				continue
			}
			n, err := l.add(p, st)
			queued += n
			if err != nil {
				errs = append(errs, err)
			}
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !IsReplay(path) {
				return nil // unreadable subfolders are skipped, not fatal
			}
			info, err := d.Info()
			if err != nil || fresh(info) {
				return nil
			}
			n, err := l.add(path, info)
			queued += n
			if err != nil {
				errs = append(errs, err)
			}
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	if queued > 0 {
		l.emitProgress()
	}
	return queued, errors.Join(errs...)
}

// Scan imports the replay folder. A missing folder is not an error (Dota may
// not be installed on this machine); it just finds nothing.
func (l *Library) Scan(dir string) (int, error) {
	if dir == "" {
		return 0, nil
	}
	l.PruneMissing()
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	// Files still being written (a game in progress) are left for the watcher.
	return l.importPaths([]string{dir}, WatchSettle)
}

// WatchSettle is how long a replay file must stay unchanged before the
// watcher picks it up: Dota writes partial "auto-…-start-…" demos during a
// game and deletes them afterwards.
const WatchSettle = time.Minute

// StatusRemoved is the ReplayEvent status of a replay forgotten because its
// file is gone before it was parsed.
const StatusRemoved = "removed"

// forget drops a replay that has no parsed match. Parsed matches stay even
// when the file is gone (their stats live in the database).
func (l *Library) forget(id int64) bool {
	has, err := l.st.HasMatch(id)
	if err != nil || has {
		return false
	}
	if err := l.st.DeleteReplay(id); err != nil {
		slog.Warn("forget replay", "replay", id, "err", err)
		return false
	}
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: StatusRemoved})
	return true
}

// PruneMissing forgets pending / failed replays whose file no longer exists.
// Returns how many were removed.
func (l *Library) PruneMissing() int {
	rs, err := l.st.UnparsedReplays()
	if err != nil {
		slog.Warn("list unparsed replays", "err", err)
		return 0
	}
	n := 0
	for _, r := range rs {
		if _, err := os.Stat(r.Path); errors.Is(err, fs.ErrNotExist) && l.forget(r.ID) {
			n++
		}
	}
	if n > 0 {
		slog.Info("forgot replays whose file is gone", "count", n)
		l.emitProgress()
	}
	return n
}

// Watch rescans the replay folder every interval while enabled() is true,
// queueing replays that appeared since. It returns when ctx is done.
func (l *Library) Watch(ctx context.Context, interval time.Duration, dir func() string, enabled func() bool) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		l.PruneMissing()
		d := dir()
		if !enabled() || d == "" {
			continue
		}
		if _, err := os.Stat(d); err != nil {
			continue
		}
		if n, err := l.importPaths([]string{d}, WatchSettle); err != nil {
			slog.Warn("watch replay folder", "dir", d, "err", err)
		} else if n > 0 {
			slog.Info("new replays in folder", "dir", d, "queued", n)
		}
	}
}

func (l *Library) add(path string, info fs.FileInfo) (int, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	id, needs, err := l.st.AddReplay(abs, info.Size(), info.ModTime().Unix())
	if err != nil {
		return 0, err
	}
	if !needs {
		return 0, nil
	}
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusPending})
	l.enqueue(id)
	return 1, nil
}

// ReparseAll queues every parsed or failed replay again (onlyStale: only
// those parsed by an older parser version, plus failed ones). Replays already
// queued or parsing are left alone. Returns how many were queued.
func (l *Library) ReparseAll(onlyStale bool) (int, error) {
	ids, err := l.st.QueueReparse(onlyStale, parser.Version)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusPending})
		l.enqueue(id)
	}
	l.emitProgress()
	return len(ids), nil
}

// ErrPruned means the replay file was removed by the storage cap.
var ErrPruned = errors.New("реплей удалён, чтобы не занимать место: перепарсить его нельзя")

// Reparse queues one replay again regardless of its status (but not a replay
// whose file the storage cap removed).
func (l *Library) Reparse(id int64) error {
	if r, err := l.st.Replay(id); err != nil {
		return err
	} else if r.PrunedAt > 0 {
		return ErrPruned
	}
	if err := l.st.SetStatus(id, store.StatusPending, ""); err != nil {
		return err
	}
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusPending})
	l.enqueue(id)
	l.emitProgress()
	return nil
}
