package stats

import (
	"testing"
	"time"

	"github.com/1rowvy/drstats/internal/store"
)

func TestTierMMR(t *testing.T) {
	for _, c := range []struct {
		tier int
		mmr  float64
	}{{11, 77}, {51, 3157}, {65, 4543}, {71, 4720}, {75, 5520}, {80, 5720}, {0, 0}} {
		if got := TierMMR(c.tier); got != c.mmr {
			t.Errorf("TierMMR(%d) = %v, want %v", c.tier, got, c.mmr)
		}
		if c.tier > 0 {
			if got := MMRTier(c.mmr); got != c.tier {
				t.Errorf("MMRTier(%v) = %d, want %d", c.mmr, got, c.tier)
			}
		}
	}
	if got := nextTierMMR(65); got != 4620 {
		t.Errorf("next(65) = %v", got)
	}
}

func TestBuildRank(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.Local)
	day := int64(24 * 3600)
	var games []Game
	// 10 games a day apart, ending now-9d .. now: W W L W W W L L W W.
	res := []bool{true, true, false, true, true, true, false, false, true, true}
	for i, w := range res {
		g := Game{Win: w}
		g.MatchID = int64(i + 1)
		g.EndTime = now.Unix() - int64(9-i)*day
		g.Duration = 1800
		g.HeroName = "a"
		if i%2 == 1 {
			g.HeroName = "b"
		}
		g.PartySize = 1
		games = append(games, g)
	}
	games[4].RankTier = 54 // Legend 4 before game 5
	v := &RankView{Days: 7}
	buildRank(v, games, nil, now)
	// Before game 5 the curve is at +50 (W W L W); Legend 4 = 3619.
	if !v.Anchored || v.Tier != 54 {
		t.Fatalf("anchor: %+v", v)
	}
	want := TierMMR(54) + float64(MMRPerGame*(4-2)) // games 5..10: W W L L W W
	if v.MMR != want {
		t.Fatalf("mmr %v, want %v", v.MMR, want)
	}
	// Last 7 days: games ending after now-7d = games 4..10 (W W W L L W W).
	if v.Games != 7 || v.Wins != 5 || v.Net != 3*MMRPerGame || len(v.Points) != 7 {
		t.Fatalf("period: games %d wins %d net %d points %d", v.Games, v.Wins, v.Net, len(v.Points))
	}
	if v.BestRun != 3 || v.WorstRun != 2 || v.Streak != 2 {
		t.Fatalf("runs %d %d %d", v.BestRun, v.WorstRun, v.Streak)
	}
	if len(v.Marks) != 1 || v.Marks[0].Index != 1 {
		t.Fatalf("marks %+v", v.Marks)
	}
	if len(v.ByParty) != 1 || v.ByParty[0].Key != "solo" || len(v.ByHero) != 2 {
		t.Fatalf("buckets %+v %+v", v.ByParty, v.ByHero)
	}

	// A newer profile medal wins.
	v = &RankView{}
	buildRank(v, games, []store.RankSeen{{T: now.Unix(), RankTier: 61}}, now)
	if v.Tier != 61 || v.MMR != TierMMR(61) || v.PeakTier != 61 {
		t.Fatalf("profile anchor: tier %d mmr %v peak %d", v.Tier, v.MMR, v.PeakTier)
	}
}
