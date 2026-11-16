package stats

import (
	"testing"
	"time"

	"github.com/1rowvy/drstats/internal/store"
)

func TestPlan(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	var games []Game
	// 30 games: deaths vary 2..9 (best quarter ~2.5), LH@10 steady at 60.
	for i := 0; i < 30; i++ {
		lh := 60
		games = append(games, Game{GameRow: store.GameRow{MatchID: int64(i + 1), EndTime: now.Unix() - int64(30-i)*3600,
			Deaths: 2 + i%8, Kills: 5, Assists: 5, TeamKills: 30, LH10: &lh}})
	}
	p := NewPlan(games, nil, now)
	if p == nil || len(p.Goals) == 0 || p.End-p.Start != PlanDays*86400 {
		t.Fatalf("plan %+v", p)
	}
	var deaths *PlanGoal
	for i := range p.Goals {
		if p.Goals[i].Key == "deaths" || p.Goals[i].Key == "deaths10" {
			if deaths != nil {
				t.Fatalf("two deaths goals: %+v", p.Goals)
			}
			deaths = &p.Goals[i]
		}
		if p.Goals[i].Key == "lh10" {
			t.Errorf("steady LH@10 picked as a goal: %+v", p.Goals[i])
		}
	}
	if deaths == nil || deaths.Key != "deaths" || deaths.Target >= deaths.Base {
		t.Fatalf("deaths goal %+v in %+v", deaths, p.Goals)
	}
	// Two games after the start: one meets the target, one does not.
	after := []Game{
		{GameRow: store.GameRow{MatchID: 100, EndTime: now.Unix() + 3600, Deaths: 1, TeamKills: 10}},
		{GameRow: store.GameRow{MatchID: 101, EndTime: now.Unix() + 7200, Deaths: 12, TeamKills: 10}},
	}
	v := PlanProgress(p, append(games, after...), nil, now.Add(3*time.Hour))
	if v.Games != 2 || v.Expired {
		t.Fatalf("view games %d expired %v", v.Games, v.Expired)
	}
	for _, g := range v.Goals {
		if g.Key == "deaths" && (len(g.Games) != 2 || g.Hits != 1 || *g.Current != 6.5) {
			t.Errorf("deaths progress %+v", g)
		}
	}
	if !PlanProgress(p, games, nil, now.AddDate(0, 0, 8)).Expired {
		t.Error("plan not expired after 8 days")
	}
}
