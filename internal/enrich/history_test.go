package enrich

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/store"
)

type fakeHistory struct {
	days []int
	err  error
}

func (f *fakeHistory) FetchHistory(_ context.Context, _ uint32, days int) ([]opendota.HistoryMatch, error) {
	f.days = append(f.days, days)
	if f.err != nil {
		return nil, f.err
	}
	return []opendota.HistoryMatch{{MatchID: int64(len(f.days)), StartTime: 1, Duration: 60, HeroID: 1}}, nil
}

func TestHistorySync(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	f := &fakeHistory{}
	var events []HistoryUpdated
	on := true
	h := NewHistory(st, f, func(_ string, d any) { events = append(events, d.(HistoryUpdated)) }, func() bool { return on })
	ctx := context.Background()

	if err := h.Sync(ctx, 0, true); err != nil || len(f.days) != 0 {
		t.Fatalf("no account: err %v, calls %v", err, f.days)
	}
	if err := h.Sync(ctx, 9, false); err != nil {
		t.Fatal(err)
	}
	if err := h.Sync(ctx, 9, false); err != nil { // fresh: skipped
		t.Fatal(err)
	}
	if err := h.Sync(ctx, 9, true); err != nil { // forced: incremental
		t.Fatal(err)
	}
	if len(f.days) != 2 || f.days[0] != 0 || f.days[1] != 2 {
		t.Fatalf("fetch days %v, want [0 2]", f.days)
	}
	f.err = errors.New("HTTP 500")
	if err := h.Sync(ctx, 9, true); err == nil {
		t.Fatal("want error")
	}
	last := events[len(events)-1].State
	if len(events) != 3 || last.Status != "error" || last.Count != 2 {
		t.Fatalf("events %+v", events)
	}
	on = false
	if err := h.Sync(ctx, 9, true); err != nil || len(f.days) != 3 {
		t.Fatalf("disabled: err %v, calls %v", err, f.days)
	}
}
