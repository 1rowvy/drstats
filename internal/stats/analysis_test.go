package stats

import (
	"path/filepath"
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

func TestIdlePeriods(t *testing.T) {
	var ss []parser.Sample
	lh := 0
	for i := 0; i <= 40; i++ {
		tm := float64(i * 5)
		// Farm until 50s, idle 50..100s, dead 100..120s, idle 120..150s (too short with 40s), farm after.
		alive := tm < 100 || tm >= 120
		if tm <= 50 || tm >= 150 {
			lh++
		}
		ss = append(ss, parser.Sample{T: tm, Alive: alive, LastHits: lh})
	}
	got := IdlePeriods(ss, 40)
	if len(got) != 1 || got[0].From != 50 || got[0].To != 95 {
		t.Fatalf("idle = %+v", got)
	}
	if got := IdlePeriods(ss, 20); len(got) != 2 {
		t.Fatalf("idle@20 = %+v", got)
	}
}

func TestIdlePeriodsGameStart(t *testing.T) {
	// Walking to the lane until 40s, farming after: not idle.
	var ss []parser.Sample
	lh := 0
	for i := 0; i <= 30; i++ {
		tm := float64(i * 5)
		if tm >= 40 {
			lh++
		}
		ss = append(ss, parser.Sample{T: tm, Alive: true, LastHits: lh})
	}
	if got := IdlePeriods(ss, 20); len(got) != 0 {
		t.Fatalf("idle = %+v", got)
	}
	// Doing nothing at all: idle counts from the end of the grace window.
	for i := range ss {
		ss[i].LastHits = 0
	}
	got := IdlePeriods(ss, 20)
	if len(got) != 1 || got[0].From != 55 {
		t.Fatalf("idle (afk) = %+v", got)
	}
}

func TestDetectLane(t *testing.T) {
	var top, mid, jungle []parser.Sample
	for i := 12; i <= 120; i++ {
		tm := float64(i * 5)
		top = append(top, parser.Sample{T: tm, Alive: true, X: -6500, Y: 3000})
		mid = append(mid, parser.Sample{T: tm, Alive: true, X: -500, Y: -300})
		jungle = append(jungle, parser.Sample{T: tm, Alive: true, X: 0, Y: -4000})
	}
	for name, c := range map[string]struct {
		ss   []parser.Sample
		want string
	}{"top": {top, LaneTop}, "mid": {mid, LaneMid}, "jungle": {jungle, LaneJungle}} {
		if got := DetectLane(c.ss); got != c.want {
			t.Errorf("%s: got %q", name, got)
		}
	}
	ts := ComputeTimeShare(top, parser.TeamRadiant, LaneTop)
	if ts.Lane != 1 {
		t.Errorf("time share %+v", ts)
	}
}

func TestAbilityHitRate(t *testing.T) {
	m := &parser.Match{
		Casts: []parser.Cast{{T: 10, PlayerID: 0, Ability: "lina_dragon_slave"}, {T: 20, PlayerID: 0, Ability: "lina_dragon_slave"},
			{T: 30, PlayerID: 0, Ability: "item_blink"}},
		Hits: []parser.Hit{{T: 10.4, PlayerID: 0, Ability: "lina_dragon_slave"}, {T: 25, PlayerID: 0, Ability: "lina_dragon_slave"}},
	}
	a := buildAbilities(m, 0, Options{HitWindow: 1})
	if len(a.Uses) != 2 || a.Uses[0].Casts != 2 || a.Uses[0].Hits != 1 || *a.Uses[0].HitRate != 0.5 {
		t.Fatalf("uses %+v", a.Uses)
	}
	if a.Uses[1].HitRate != nil {
		t.Error("blink never hit anything: hit rate must be nil")
	}
}

// TestMatchViewOnReplay builds the full view for every player of a real replay.
func TestMatchViewOnReplay(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "*.dem"))
	if len(files) == 0 {
		t.Skip("no replay in testdata/, skipping")
	}
	m, err := parser.ParseFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, _, _ := st.AddReplay(files[0], 1, 1)
	mid, err := st.SaveMatch(id, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range m.Players {
		v, err := BuildMatchView(st, mid, 0, p.PlayerID, DefaultOptions)
		if err != nil {
			t.Fatal(err)
		}
		share := v.Vision.TimeShare
		t.Logf("%-30s lane %-6s opp %d (%s) kp %.2f idle %.0fs/%d abilities %d items %d share b%.2f l%.2f j%.2f e%.2f",
			p.HeroName, v.Laning.Lane, v.Opponent, v.Laning.OppLane, v.Fights.KP, v.Gold.IdleTotal, len(v.Gold.Idle),
			len(v.Abilities.Uses), len(v.Items), share.Base, share.Lane, share.Jungle, share.Enemy)
		if v.Laning.Lane == "" || len(v.Laning.Snaps) != 2 || len(v.Abilities.Uses) == 0 || len(v.Items) == 0 {
			t.Errorf("%s: incomplete view", p.HeroName)
		}
		if sum := share.Base + share.Lane + share.Jungle + share.Enemy; sum < 0.99 || sum > 1.01 {
			t.Errorf("%s: time share sums to %v", p.HeroName, sum)
		}
		var won, kills int
		for _, b := range v.Battles {
			if b.Won {
				won++
			}
			kills += b.MyKills
		}
		t.Logf("    battles %d (won %d), my kills in battles %d/%d", len(v.Battles), won, kills, p.Kills)
		if len(v.Battles) == 0 || kills > p.Kills {
			t.Errorf("%s: battles %d, kills %d of %d", p.HeroName, len(v.Battles), kills, p.Kills)
		}
	}
	// Stored-death classification matches the one of the match view.
	for _, p := range m.Players {
		if p.AccountID == 0 {
			continue
		}
		scenes, err := st.DeathScenes(p.AccountID)
		if err != nil {
			t.Fatal(err)
		}
		v, _ := BuildMatchView(st, mid, 0, p.PlayerID, DefaultOptions)
		var want []DeathCtx
		for i, d := range v.Deaths {
			if d.VictimPID == p.PlayerID {
				c := v.DeathCtx[i]
				c.Tick = 0
				want = append(want, c)
			}
		}
		got := classifyScenes(scenes)[mid]
		if len(got) != len(want) {
			t.Fatalf("%s: %d stored deaths, %d in view", p.HeroName, len(got), len(want))
		}
		avoid := 0
		for i := range got {
			if got[i].Ctx != want[i] {
				t.Errorf("%s death %d: stored %+v, view %+v", p.HeroName, i, got[i].Ctx, want[i])
			}
			if got[i].Ctx.Avoidable {
				avoid++
			}
		}
		t.Logf("%-30s deaths %d, avoidable %d", p.HeroName, len(got), avoid)
	}

	v, _ := BuildMatchView(st, mid, 0, 0, DefaultOptions)
	adv := v.Farm.Advantage
	last := adv[len(adv)-1]
	if last == nil || (*last > 0) != (m.Winner == parser.TeamRadiant) {
		t.Errorf("final advantage %v, winner %d", last, m.Winner)
	}
	if len(v.Objectives) != 21 || len(v.Farm.Ticks) != len(v.Farm.T) {
		t.Errorf("objectives %d, ticks %d/%d", len(v.Objectives), len(v.Farm.Ticks), len(v.Farm.T))
	}
}
