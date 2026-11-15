package store

import (
	"testing"

	"github.com/1rowvy/drstats/internal/opendota"
)

func TestHistory(t *testing.T) {
	s := openTemp(t)
	if h, err := s.HistoryState(7); err != nil || h.Status != "" || h.Count != 0 {
		t.Fatalf("empty state = %+v, %v", h, err)
	}
	ms := []opendota.HistoryMatch{
		{MatchID: 2, StartTime: 200, Duration: 1500, HeroID: 5, RadiantWin: true, PlayerSlot: 1, Kills: 3},
		{MatchID: 1, StartTime: 100, Duration: 1600, HeroID: 6, PlayerSlot: 129, PartySize: 3, IsRoaming: true},
	}
	if err := s.SaveHistory(7, ms); err != nil {
		t.Fatal(err)
	}
	ms[0].Kills = 4 // re-sync updates in place
	if err := s.SaveHistory(7, ms[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := s.History(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].MatchID != 1 || !got[0].IsRoaming || got[0].PartySize != 3 || got[1].Kills != 4 || !got[1].RadiantWin {
		t.Fatalf("history = %+v", got)
	}
	if other, _ := s.History(8); len(other) != 0 {
		t.Fatalf("other account sees %d matches", len(other))
	}
	if err := s.HistorySyncFailed(7, "boom"); err != nil {
		t.Fatal(err)
	}
	h, err := s.HistoryState(7)
	if err != nil || h.Status != "error" || h.Error != "boom" || h.OKAt == 0 || h.Count != 2 {
		t.Fatalf("state = %+v, %v", h, err)
	}
}
