package library

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

func TestImportAndParse(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	replays := filepath.Join(dir, "replays")
	os.MkdirAll(filepath.Join(replays, "sub"), 0o755)
	for _, n := range []string{"1.dem", "sub/2.dem", "bad.dem", "notes.txt"} {
		os.WriteFile(filepath.Join(replays, n), []byte("x"), 0o644)
	}

	var mu sync.Mutex
	events := map[string]int{}
	l := New(st, func(name string, data any) {
		mu.Lock()
		defer mu.Unlock()
		if ev, ok := data.(ReplayEvent); ok {
			events[ev.Status]++
		}
	})
	l.parse = func(path string, _ func(float64)) (*parser.Match, error) {
		if strings.HasSuffix(path, "bad.dem") {
			panic("manta exploded") // must be contained
		}
		id := uint64(1)
		if strings.HasSuffix(path, "2.dem") {
			id = 2
		}
		return &parser.Match{MatchID: id, Players: []parser.Player{{PlayerID: 0, Team: 2}}}, nil
	}
	if err := l.Start(2); err != nil {
		t.Fatal(err)
	}
	defer l.Stop()

	n, err := l.Scan(replays)
	if err != nil || n != 3 {
		t.Fatalf("scan: %d %v", n, err)
	}
	l.Wait()
	if n, _ := l.Scan(replays); n != 0 {
		t.Errorf("rescan queued %d unchanged files", n)
	}
	p := l.Progress()
	if p.Done != 2 || p.Failed != 1 || p.Pending != 0 {
		t.Errorf("progress %+v", p)
	}
	rows, _ := st.ListMatches(0)
	var failed *store.MatchRow
	for i := range rows {
		if rows[i].Status == store.StatusFailed {
			failed = &rows[i]
		}
	}
	if failed == nil || !strings.Contains(failed.Error, "manta exploded") {
		t.Errorf("failed row: %+v", failed)
	}
	if events[store.StatusDone] != 2 || events[store.StatusFailed] != 1 {
		t.Errorf("events %v", events)
	}
	if _, err := l.Import([]string{filepath.Join(replays, "notes.txt")}); err == nil {
		t.Error("importing a non-replay must report an error")
	}

	// Everything is current: "only stale" re-queues just the failed replay.
	if n, err := l.ReparseAll(true); err != nil || n != 1 {
		t.Errorf("reparse stale: %d %v", n, err)
	}
	l.Wait()
	if n, _ := l.ReparseAll(false); n != 3 {
		t.Errorf("reparse all: %d", n)
	}
	l.Wait()
	if p := l.Progress(); p.Done != 2 || p.Failed != 1 || p.Stale != 0 {
		t.Errorf("after reparse: %+v", p)
	}
}
