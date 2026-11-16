package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/store"
)

func TestBuildReasons(t *testing.T) {
	var games []Game
	lh := func(v int) *int { return &v }
	// 20 games on one hero: wins have ~3 deaths and 60 LH@10, losses ~8 deaths.
	for i := 0; i < 20; i++ {
		win := i%2 == 0
		d, l := 3+i%3, 58+i%5
		if !win {
			d = 8 + i%3
		}
		games = append(games, Game{GameRow: store.GameRow{MatchID: int64(i + 1), HeroID: 1, Duration: 2400,
			Deaths: d, LH10: lh(l), TotalGold: 20000 + i*100, TeamKills: 30, Kills: 5, Assists: 10}, Win: win})
	}
	// The game to explain: a loss with many deaths and normal farm.
	games = append(games, Game{GameRow: store.GameRow{MatchID: 99, HeroID: 1, Duration: 2400, Deaths: 11, LH10: lh(60),
		TotalGold: 21000, TeamKills: 30, Kills: 5, Assists: 10}})
	ex := buildExtrasMap(games, nil, nil)
	r := buildReasons(games, ex, 99)
	if r == nil || r.Win || !r.RefWins || !r.OnHero || r.Basis != 10 {
		t.Fatalf("reasons %+v", r)
	}
	if len(r.Main) == 0 || r.Main[0].Key != "deaths" || r.Main[0].Z >= 0 || r.Main[0].WinGood == nil {
		t.Fatalf("main %+v", r.Main)
	}
	for _, x := range r.Main {
		if x.Key == "lh10" {
			t.Errorf("normal LH@10 given as a reason: %+v", x)
		}
	}
	if buildReasons(games, ex, 12345) != nil {
		t.Error("unknown match explained")
	}
}
