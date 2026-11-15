package autodl

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/1rowvy/drstats/internal/download"
	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/settings"
	"github.com/1rowvy/drstats/internal/store"
)

type fakeFetch struct {
	errs     map[uint64]error
	fetched  []uint64
	requests []uint64
}

func (f *fakeFetch) Fetch(_ context.Context, id uint64, dir string) (string, error) {
	f.fetched = append(f.fetched, id)
	if err := f.errs[id]; err != nil {
		return "", err
	}
	p := filepath.Join(dir, strconv.FormatUint(id, 10)+".dem")
	if err := os.WriteFile(p, make([]byte, 1000), 0o644); err != nil {
		return "", err
	}
	// File time = game time, like the match end time of a parsed replay.
	at := time.Now().Add(-time.Duration(id) * time.Hour)
	return p, os.Chtimes(p, at, at)
}

func (f *fakeFetch) RequestParse(_ context.Context, id uint64) error {
	f.requests = append(f.requests, id)
	return nil
}

func TestRoundAndPrune(t *testing.T) {
	tmp := t.TempDir()
	st, err := store.Open(filepath.Join(tmp, "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	Gap = 0
	dir := filepath.Join(tmp, "auto")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	var hist []opendota.HistoryMatch
	for i := int64(1); i <= 5; i++ {
		hist = append(hist, opendota.HistoryMatch{MatchID: i, StartTime: now - i*3600, HeroID: 1})
	}
	if err := st.SaveHistory(7, hist); err != nil {
		t.Fatal(err)
	}
	f := &fakeFetch{errs: map[uint64]error{4: download.ErrNoSalt, 5: download.ErrExpired}}
	var added []string
	addFn := func(p string) error {
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		id, _, err := st.AddReplay(p, info.Size(), info.ModTime().Unix())
		if err == nil {
			err = st.SetStatus(id, store.StatusDone, "") // pretend it was parsed
		}
		added = append(added, p)
		return err
	}
	pol := Policy{Keep: settings.KeepCap, CapBytes: 2500}
	on := true
	d := New(st, f, dir, addFn, nil, func() bool { return on }, func() uint32 { return 7 }, func() Policy { return pol })

	d.round(context.Background())
	if len(added) != 3 || len(f.requests) != 1 || f.requests[0] != 4 {
		t.Fatalf("added %v, parse requests %v", added, f.requests)
	}
	s, err := d.State()
	if err != nil || s.Done != 3 || s.Waiting != 1 || s.Expired != 1 || s.StoredFiles != 3 || s.StoredBytes != 3000 {
		t.Fatalf("state %+v, %v", s, err)
	}
	// Next round: nothing new (no-salt waits for RetryAfter, expired is final).
	f.fetched = nil
	d.round(context.Background())
	if len(f.fetched) != 0 {
		t.Fatalf("refetched %v", f.fetched)
	}

	// A file outside the folder must never be touched.
	outside := filepath.Join(tmp, "dota", "9.dem")
	os.MkdirAll(filepath.Dir(outside), 0o755)
	os.WriteFile(outside, make([]byte, 5000), 0o644)
	oid, _, _ := st.AddReplay(outside, 5000, 1)
	st.SetStatus(oid, store.StatusDone, "")

	// Cap 2500 bytes, 3 files of 1000: the oldest goes.
	if n, err := d.Prune(); err != nil || n != 1 {
		t.Fatalf("prune cap: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "3.dem")); !os.IsNotExist(err) {
		t.Fatal("oldest replay (match 3) not removed")
	}
	pol.Keep = settings.KeepDelete
	if n, err := d.Prune(); err != nil || n != 2 {
		t.Fatalf("prune delete: %d %v", n, err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("file outside the folder was removed")
	}
	if s, _ := d.State(); s.StoredFiles != 0 || s.StoredBytes != 0 {
		t.Fatalf("after delete: %+v", s)
	}
	on = false
	f.fetched = nil
	d.Wake()
	d.round(context.Background())
	if len(f.fetched) != 0 {
		t.Fatal("disabled downloader fetched")
	}
}
