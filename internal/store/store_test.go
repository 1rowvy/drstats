package store

import (
	"path/filepath"
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleMatch() *parser.Match {
	return &parser.Match{
		MatchID: 42, GameMode: 22, LobbyType: 7, Winner: parser.TeamDire, Duration: 1800, EndTime: 1700000000,
		Players: []parser.Player{
			{PlayerID: 0, Team: 2, HeroID: 1, HeroName: "npc_dota_hero_antimage", AccountID: 100, Kills: 3, Deaths: 1},
			{PlayerID: 5, Team: 3, HeroID: 2, HeroName: "npc_dota_hero_axe", AccountID: 200, Kills: 1, Deaths: 3},
		},
		Series: []parser.Sample{
			{T: 0, PlayerID: 0, Alive: true, NetWorth: 600},
			{T: 5, PlayerID: 0, Alive: true, NetWorth: 650},
		},
		Deaths: []parser.Death{
			{T: 100, VictimPID: 5, KillerPID: 0, Killer: "npc_dota_hero_antimage", Assists: []int{}, X: 1, Y: 2, HasPos: true},
			{T: 200, VictimPID: 0, KillerPID: -1, Killer: "npc_dota_badguys_tower1_mid"},
		},
	}
}

func TestReplayLifecycle(t *testing.T) {
	s := openTemp(t)
	id, needs, err := s.AddReplay("/r/42.dem", 10, 1)
	if err != nil || !needs {
		t.Fatalf("add: %v %v", needs, err)
	}
	if _, needs, _ := s.AddReplay("/r/42.dem", 10, 1); !needs {
		t.Error("still pending, must need parse")
	}
	if err := s.SetStatus(id, StatusFailed, "boom"); err != nil {
		t.Fatal(err)
	}
	if _, needs, _ := s.AddReplay("/r/42.dem", 10, 1); needs {
		t.Error("unchanged failed file must not be re-queued")
	}
	if _, needs, _ := s.AddReplay("/r/42.dem", 11, 2); !needs {
		t.Error("changed file must be re-queued")
	}

	mid, err := s.SaveMatch(id, sampleMatch())
	if err != nil || mid != 42 {
		t.Fatalf("save: %d %v", mid, err)
	}
	// Saving again (re-parse) replaces rows instead of failing on keys.
	if _, err := s.SaveMatch(id, sampleMatch()); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListMatches(100)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list: %+v %v", rows, err)
	}
	if r := rows[0]; !r.Found || r.HeroID != 1 || r.Kills != 3 || r.Status != StatusDone || r.Stale {
		t.Errorf("row: %+v", r)
	}
	if rows, _ := s.ListMatches(999); rows[0].Found {
		t.Error("unknown account must not be found")
	}
	meta, players, series, deaths, err := s.Match(42)
	if err != nil || meta.Winner != parser.TeamDire || len(players) != 2 || len(series) != 2 || len(deaths) != 2 {
		t.Fatalf("match: %+v %d %d %d %v", meta, len(players), len(series), len(deaths), err)
	}
	if !deaths[0].HasPos || deaths[1].HasPos || deaths[1].KillerPID != -1 {
		t.Errorf("deaths: %+v", deaths)
	}

	// A duplicate file of the same match takes it over; the old replay row is hidden.
	id2, _, _ := s.AddReplay("/other/42.dem", 10, 1)
	if _, err := s.SaveMatch(id2, sampleMatch()); err != nil {
		t.Fatal(err)
	}
	rows, _ = s.ListMatches(100)
	if len(rows) != 1 || rows[0].ReplayID != id2 {
		t.Errorf("duplicate: %+v", rows)
	}

	if err := s.DeleteReplay(id2); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := s.Match(42); err == nil {
		t.Error("match must be gone with its replay")
	}
}

func TestSettings(t *testing.T) {
	s := openTemp(t)
	var v struct{ A int }
	if ok, _ := s.GetSetting("x", &v); ok {
		t.Fatal("absent key reported present")
	}
	if err := s.SetSetting("x", struct{ A int }{7}); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.GetSetting("x", &v); !ok || err != nil || v.A != 7 {
		t.Fatalf("%v %v %v", ok, err, v)
	}
}
