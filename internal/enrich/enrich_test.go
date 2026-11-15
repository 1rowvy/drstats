package enrich

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/store"
)

type fake struct{ calls []uint64 }

func (f *fake) Fetch(_ context.Context, id uint64) (*opendota.Match, error) {
	f.calls = append(f.calls, id)
	if id == 2 {
		return nil, opendota.ErrNotFound
	}
	return &opendota.Match{MatchID: id, Parsed: true, Players: []opendota.Player{{HeroID: 7, Position: 3, RankTier: 55}}}, nil
}

func TestEnrich(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	Gap = time.Millisecond
	f := &fake{}
	done := make(chan Updated, 4)
	e := New(st, f, func(_ string, d any) { done <- d.(Updated) }, func() bool { return true })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)
	e.Queue(1, 2, 1, 0)
	got := map[int64]string{}
	for i := 0; i < 2; i++ {
		select {
		case u := <-done:
			got[u.MatchID] = u.Status
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	}
	if got[1] != opendota.StatusOK || got[2] != opendota.StatusNotFound || len(f.calls) != 2 {
		t.Fatalf("got %v, calls %v", got, f.calls)
	}
	ps, err := st.OpenDotaPlayers(1)
	if err != nil || ps[7].Position != 3 || ps[7].RankTier != 55 {
		t.Fatalf("players %v err %v", ps, err)
	}
}
