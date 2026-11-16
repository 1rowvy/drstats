package store

import (
	"path/filepath"
	"reflect"
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
			{T: 200, VictimPID: 0, KillerPID: -1, Killer: "npc_dota_badguys_tower1_mid", GoldLost: 120, Buyback: true},
		},
		Purchases: []parser.Purchase{{T: 30, PlayerID: 0, Item: "blink"}},
		Casts:     []parser.Cast{{T: 40, PlayerID: 0, Ability: "antimage_blink"}},
		Hits:      []parser.Hit{{T: 41, PlayerID: 0, Ability: "antimage_mana_void"}},
		Combat:    []parser.CombatStat{{PlayerID: 0, Kind: parser.DmgOut, Other: "npc_dota_hero_axe", Amount: 500, Count: 3}},
		Wards: []parser.Ward{{T: 50, End: 60, PlayerID: 5, Team: 3, Sentry: true, X: 1, Y: 2, Destroyed: true,
			KillerPID: 0, Killer: "npc_dota_hero_antimage"}},
		Runes:    []parser.Rune{{T: 0, PlayerID: 0, Type: parser.RuneBounty, Bottled: true}},
		Stacks:   []parser.Mark{{T: 70, PlayerID: 5}},
		Buybacks: []parser.Mark{{T: 210, PlayerID: 0}},
		Objectives: []parser.Objective{{T: 300, Kind: "tower", Key: "npc_dota_badguys_tower1_mid", Team: 3, KillerPID: 0,
			Killer: "npc_dota_hero_antimage"}},
		HeroDamage: []parser.DamageBucket{{T: 95, PlayerID: 0, Target: 5, Inflictor: "", Amount: 240}},
		Pauses:     [][2]uint32{{4000, 4300}},
		Smokes:     []parser.Smoke{{T: 400, End: 420.5, PlayerID: 0, UserPID: 0}, {T: 400, End: 431, PlayerID: 1, UserPID: 0}},
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
	meta, mm, err := s.Match(42)
	if err != nil {
		t.Fatal(err)
	}
	players, series, deaths := mm.Players, mm.Series, mm.Deaths
	if meta.Winner != parser.TeamDire || len(players) != 2 || len(series) != 2 || len(deaths) != 2 {
		t.Fatalf("match: %+v %d %d %d", meta, len(players), len(series), len(deaths))
	}
	if !deaths[0].HasPos || deaths[1].HasPos || deaths[1].KillerPID != -1 {
		t.Errorf("deaths: %+v", deaths)
	}
	want := sampleMatch()
	if !reflect.DeepEqual(deaths[1], want.Deaths[1]) {
		t.Errorf("death round trip: %+v", deaths[1])
	}
	for name, pair := range map[string][2]any{
		"purchases": {mm.Purchases, want.Purchases}, "casts": {mm.Casts, want.Casts}, "hits": {mm.Hits, want.Hits},
		"combat": {mm.Combat, want.Combat}, "wards": {mm.Wards, want.Wards}, "runes": {mm.Runes, want.Runes},
		"stacks": {mm.Stacks, want.Stacks}, "objectives": {mm.Objectives, want.Objectives},
		"hero_damage": {mm.HeroDamage, want.HeroDamage}, "pauses": {mm.Pauses, want.Pauses}, "buybacks": {mm.Buybacks, want.Buybacks}, "smokes": {mm.Smokes, want.Smokes}, "players": {mm.Players, want.Players},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s round trip:\n got %+v\nwant %+v", name, pair[0], pair[1])
		}
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
	if _, _, err := s.Match(42); err == nil {
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

func TestAggregateQueries(t *testing.T) {
	s := openTemp(t)
	id, _, _ := s.AddReplay("/r/42.dem", 10, 1)
	m := sampleMatch()
	m.Series = append(m.Series, parser.Sample{T: 600, PlayerID: 0, Alive: true, LastHits: 55, NetWorth: 4000})
	if _, err := s.SaveMatch(id, m); err != nil {
		t.Fatal(err)
	}
	games, err := s.Games(100)
	if err != nil || len(games) != 1 {
		t.Fatalf("games: %+v %v", games, err)
	}
	g := games[0]
	if g.HeroID != 1 || g.TeamKills != 3 || g.LH10 == nil || *g.LH10 != 55 || g.Deaths10 != 1 || len(g.LaneEarly) != 1 {
		t.Errorf("game row: %+v", g)
	}
	if d, _ := s.DeathPoints(200, 0); len(d) != 1 || d[0].X != 1 {
		t.Errorf("death points: %+v", d)
	}
	if it, _ := s.ItemTimings(100, 1, 0); len(it) != 1 || it[0].Item != "blink" || it[0].Avg != 30 {
		t.Errorf("item timings: %+v", it)
	}
	if ab, _ := s.AbilityCasts(100, 1); len(ab) != 1 || ab[0].Casts != 1 {
		t.Errorf("ability casts: %+v", ab)
	}
}
