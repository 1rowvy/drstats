package store

import (
	"testing"
	"time"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/parser"
)

func TestMissingReplaysAndPruning(t *testing.T) {
	s := openTemp(t)
	now := time.Now().Unix()
	// 42 has a stored match, 43 a final outcome, 44 a fresh failure, 45 an old
	// failure, 46 is missing, 47 is too old.
	var hist []opendota.HistoryMatch
	for i, id := range []int64{42, 43, 44, 45, 46} {
		hist = append(hist, opendota.HistoryMatch{MatchID: id, StartTime: now - int64(i)*3600, HeroID: 1})
	}
	hist = append(hist, opendota.HistoryMatch{MatchID: 47, StartTime: now - 30*86400, HeroID: 1})
	if err := s.SaveHistory(100, hist); err != nil {
		t.Fatal(err)
	}
	dir := "/data/Имя/replays/auto"
	rid, _, err := s.AddReplay(dir+"/42.dem", 64<<20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveMatch(rid, sampleMatch()); err != nil {
		t.Fatal(err)
	}
	other, _, _ := s.AddReplay("/data/Имя/replays/autoX/9.dem", 1, 1) // sibling folder: not inside dir
	_ = other
	for id, st := range map[int64]string{43: AutoExpired, 44: AutoError, 45: AutoNoSalt} {
		if err := s.SetAutoDownload(id, st, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`UPDATE auto_downloads SET tried_at = 0 WHERE match_id = 45`); err != nil {
		t.Fatal(err)
	}
	got, err := s.MissingReplays(100, now-15*86400, now-3600)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 45 || got[1] != 46 {
		t.Fatalf("missing = %v, want [45 46]", got)
	}

	in, err := s.ReplaysIn(dir + "/")
	if err != nil || len(in) != 1 || in[0].ID != rid || in[0].Status != StatusDone {
		t.Fatalf("replays in dir = %+v, %v", in, err)
	}
	if err := s.MarkPruned(rid); err != nil {
		t.Fatal(err)
	}
	if in, _ := s.ReplaysIn(dir); len(in) != 0 {
		t.Fatalf("pruned replay still listed: %+v", in)
	}
	ids, err := s.QueueReparse(false, parser.Version)
	if err != nil || len(ids) != 0 {
		t.Fatalf("pruned replay queued for re-parse: %v %v", ids, err)
	}
	rows, err := s.ListMatches(100)
	if err != nil || len(rows) == 0 || !rows[0].Pruned || rows[0].Stale {
		t.Fatalf("list = %+v, %v", rows, err)
	}
}
