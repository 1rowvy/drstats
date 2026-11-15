// Package library finds replay files, registers them in the store and parses
// them in a background worker pool, reporting progress through an emit hook.
package library

import (
	"errors"
	"fmt"
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
)

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
}

// ParseFunc parses one replay file; replaceable in tests.
type ParseFunc func(path string) (*parser.Match, error)

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
	l := &Library{st: st, emit: emit, parse: parser.ParseFile, queued: map[int64]bool{}, limit: 1}
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
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusParsing})
	l.emitProgress()

	started := time.Now()
	m, err := l.parse(r.Path)
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
	c, err := l.st.StatusCounts()
	if err != nil {
		return
	}
	l.emit(EventProgress, Progress{Pending: c[store.StatusPending], Parsing: c[store.StatusParsing],
		Done: c[store.StatusDone], Failed: c[store.StatusFailed]})
}

// Progress returns the current queue state.
func (l *Library) Progress() Progress {
	c, _ := l.st.StatusCounts()
	return Progress{Pending: c[store.StatusPending], Parsing: c[store.StatusParsing],
		Done: c[store.StatusDone], Failed: c[store.StatusFailed]}
}

// IsReplay reports whether a path looks like a Dota 2 replay file.
func IsReplay(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".dem")
}

// Import registers files and/or directories (walked recursively) and queues
// new or changed replays. It returns how many replays were queued.
func (l *Library) Import(paths []string) (int, error) {
	queued := 0
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
			if err != nil {
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
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return l.Import([]string{dir})
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

// Reparse queues one replay again regardless of its status.
func (l *Library) Reparse(id int64) error {
	if err := l.st.SetStatus(id, store.StatusPending, ""); err != nil {
		return err
	}
	l.emit(EventReplay, ReplayEvent{ReplayID: id, Status: store.StatusPending})
	l.enqueue(id)
	l.emitProgress()
	return nil
}
