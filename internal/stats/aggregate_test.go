package stats

import (
	"testing"
	"time"

	"github.com/1rowvy/drstats/internal/store"
)

func games(n int, mk func(i int) store.GameRow) []Game {
	out := make([]Game, n)
	for i := range out {
		r := mk(i)
		out[i] = Game{GameRow: r, Win: r.Team == r.Winner}
	}
	return out
}

func TestHabits(t *testing.T) {
	// 20 matches: deaths 2 for the first 10, then 6; kills stay at 5.
	gs := games(20, func(i int) store.GameRow {
		d := 2
		if i >= 10 {
			d = 6
		}
		return store.GameRow{MatchID: int64(i), Kills: 5, Deaths: d, Duration: 1800, TotalGold: 15000, Team: 2, Winner: 2}
	})
	hs := habits(gs)
	if len(hs) == 0 || hs[0].Key != "deaths" && hs[0].Key != "kda" || !hs[0].Worse {
		t.Fatalf("habits: %+v", hs)
	}
	for _, h := range hs {
		if h.Key == "kills" && h.Worse {
			t.Error("unchanged kills flagged as worse")
		}
		if h.Key == "deaths" && (!h.Worse || h.Recent != 6 || h.Before != 2) {
			t.Errorf("deaths habit %+v", h)
		}
	}
	if len(habits(gs[:12])) != 0 {
		t.Error("too few matches must give no habits")
	}
}

func TestApplyFilter(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	gs := games(10, func(i int) store.GameRow {
		return store.GameRow{MatchID: int64(i), HeroID: 1 + i%2, EndTime: now.Unix() - int64(10-i)*86400, Team: 2, Winner: 2 + i%3/2, GameMode: 22}
	})
	if got := applyFilter(gs, Filter{HeroID: 2}, now); len(got) != 5 {
		t.Errorf("hero: %d", len(got))
	}
	if got := applyFilter(gs, Filter{Days: 3}, now); len(got) != 3 {
		t.Errorf("days: %d", len(got))
	}
	if got := applyFilter(gs, Filter{Last: 4, HeroID: 1}, now); len(got) != 4 || got[3].MatchID != 8 {
		t.Errorf("last: %+v", got)
	}
	wins := applyFilter(gs, Filter{Result: "win"}, now)
	losses := applyFilter(gs, Filter{Result: "loss"}, now)
	if len(wins)+len(losses) != 10 || len(losses) == 0 {
		t.Errorf("result: %d / %d", len(wins), len(losses))
	}
	if got := applyFilter(gs, Filter{Mode: 23}, now); len(got) != 0 {
		t.Errorf("mode: %d", len(got))
	}
}

func TestGradesAndExtras(t *testing.T) {
	// 12 games on one hero, GPM rising; the last one is the best and a win.
	gs := games(12, func(i int) store.GameRow {
		w := 3
		if i%2 == 0 || i == 11 {
			w = 2
		}
		return store.GameRow{MatchID: int64(i + 1), HeroID: 1, Team: 2, Winner: w, Duration: 1800,
			EndTime: time.Now().Add(-time.Duration(12-i) * time.Hour).Unix(),
			TotalGold: 9000 + 600*i, TotalXP: 9000 + 600*i, Kills: i, Deaths: 12 - i, Assists: 5, HeroDamage: 1000 * i, TeamKills: 30}
	})
	g := Grades(gs)
	best, worst := g[12], g[1]
	if best.Letter == "" || best.Score < 90 || !best.OnHero || best.Basis != 11 {
		t.Errorf("best: %+v", best)
	}
	if worst.Score > 20 {
		t.Errorf("worst: %+v", worst)
	}
	x := buildExtras(gs, time.Now())
	if x.Hours != 6 || x.Streak.Len != 2 || len(x.Records) != 4 || x.Records[0].MatchID != 12 {
		t.Errorf("extras: hours %v streak %+v records %+v", x.Hours, x.Streak, x.Records)
	}
}
