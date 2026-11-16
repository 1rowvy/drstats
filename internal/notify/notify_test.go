package notify

import (
	"strings"
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/stats"
	"github.com/1rowvy/drstats/internal/store"
)

func TestBuild(t *testing.T) {
	v := &stats.MatchView{
		Meta:  store.MatchMeta{MatchID: 7, Winner: parser.TeamDire, Duration: 2052},
		Focus: 1,
		Players: []stats.PlayerView{
			{Player: parser.Player{PlayerID: 0, Team: parser.TeamRadiant}},
			{Player: parser.Player{PlayerID: 1, Team: parser.TeamRadiant, Kills: 4, Deaths: 9, Assists: 6}, GPM: 412},
		},
		Grade:   &stats.Grade{Letter: "C-"},
		Reasons: &stats.MatchReasons{Main: []stats.Reason{{Key: "lh10", Value: 31, Ref: 46}}},
	}
	plan := stats.PlanView{
		Plan: &stats.Plan{},
		Goals: []stats.GoalProgress{
			{PlanGoal: stats.PlanGoal{Key: "deaths"}, Games: []stats.GoalGame{{MatchID: 7, Met: false}}},
			{PlanGoal: stats.PlanGoal{Key: "kp"}, Games: []stats.GoalGame{{MatchID: 6, Met: false}, {MatchID: 7, Met: true}}},
		},
	}
	m := Build(v, plan, "Sniper")
	if m.Title != "Поражение · Sniper · оценка C-" {
		t.Errorf("title %q", m.Title)
	}
	for _, want := range []string{"4/9/6 · 34:12 · 412 GPM", "Проиграно на: добивания к 10:00 — 31 (обычно 46)", "План: 1 из 2 целей · мимо: смерти"} {
		if !strings.Contains(m.Body, want) {
			t.Errorf("body %q lacks %q", m.Body, want)
		}
	}
	// No focus: nothing to announce.
	v.Focus = 5
	if m := Build(v, plan, "Sniper"); m.Title != "" {
		t.Errorf("unexpected %+v", m)
	}
}
