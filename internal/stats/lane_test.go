package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

func TestLaneOutcome(t *testing.T) {
	for _, c := range []struct {
		nw, oppNW, xp, oppXP int
		want                 string
	}{
		{3000, 2400, 3000, 3000, LaneWon},
		{3000, 2800, 3000, 2000, LaneWon}, // 200 + 500
		{3000, 3200, 3000, 3200, LaneEven},
		{2000, 2700, 3000, 3000, LaneLost},
	} {
		if got := LaneOutcome(LaneScore(c.nw, c.oppNW, c.xp, c.oppXP)); got != c.want {
			t.Errorf("%+v: got %s", c, got)
		}
	}
}

func TestAttachLaneOppsAndStats(t *testing.T) {
	// Me at (0,0) every 30 s; rival 5 stands next to me, rival 6 far away.
	track := func(pid int, x float64, nw10, xp10 int) []parser.Sample {
		var ss []parser.Sample
		for tt := 60.0; tt <= 600; tt += 30 {
			ss = append(ss, parser.Sample{T: tt, PlayerID: pid, X: x, Alive: true, NetWorth: nw10, TotalXP: xp10})
		}
		return ss
	}
	nw, xp := 3000, 4000
	g := func(id int64, win bool) Game {
		return Game{GameRow: store.GameRow{MatchID: id, NW10: &nw, XP10: &xp, LaneEarly: track(0, 0, 0, 0)}, Win: win}
	}
	games := []Game{g(1, true), g(2, false), {GameRow: store.GameRow{MatchID: 3}, OpenDota: true}}
	rivals := map[int64][]store.Rival{
		1: {{PlayerID: 5, HeroName: "npc_dota_hero_axe", Samples: track(5, 300, 2000, 4000)},
			{PlayerID: 6, HeroName: "npc_dota_hero_lion", Samples: track(6, 9000, 9000, 9000)}},
		2: {{PlayerID: 5, HeroName: "npc_dota_hero_axe", Samples: track(5, 300, 3600, 4400)}},
	}
	attachLaneOpps(games, rivals)
	if games[0].Opp == nil || games[0].Opp.HeroName != "npc_dota_hero_axe" || games[0].LaneResult != LaneWon {
		t.Fatalf("game 1: %+v %s", games[0].Opp, games[0].LaneResult)
	}
	if games[1].LaneResult != LaneLost || games[2].Opp != nil {
		t.Fatalf("game 2: %s, game 3 opp %+v", games[1].LaneResult, games[2].Opp)
	}
	ls := laneStats(games)
	if ls.Games != 2 || ls.Won.Games != 1 || ls.Won.Wins != 1 || ls.Lost.Games != 1 || ls.Lost.Wins != 0 {
		t.Fatalf("lane stats %+v", ls)
	}
	if len(ls.Matchups) != 1 || ls.Matchups[0].Games != 2 || ls.Matchups[0].Won != 1 || ls.Matchups[0].Lost != 1 || ls.AvgNW != 200 {
		t.Fatalf("matchups %+v avg %v", ls.Matchups, ls.AvgNW)
	}
}
