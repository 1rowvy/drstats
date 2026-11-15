package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/store"
)

func TestMergeHistory(t *testing.T) {
	replays := games(2, func(i int) store.GameRow {
		return store.GameRow{MatchID: int64(10 + i), EndTime: int64(5000 * (i + 1)), Duration: 1800, Team: 2, Winner: 2, HeroID: 1,
			HeroName: "npc_dota_hero_antimage", CampsStacked: 3}
	})
	hist := []opendota.HistoryMatch{
		{MatchID: 10, StartTime: 0, Duration: 1800, HeroID: 1, PartySize: 2, RadiantWin: true}, // has a replay
		{MatchID: 5, StartTime: 100, Duration: 1200, HeroID: 2, PlayerSlot: 130, RadiantWin: false,
			GoldPerMin: 500, HeroDamage: 0, Lane: 3, PartySize: 1, Kills: 7},
	}
	names := HeroNamer(map[int]string{2: "npc_dota_hero_axe"})

	only := mergeHistory(replays, hist, names, false)
	if len(only) != 2 || only[0].PartySize != 2 || only[1].PartySize != 0 {
		t.Fatalf("replays only: %+v", only)
	}

	all := mergeHistory(replays, hist, names, true)
	if len(all) != 3 || all[0].MatchID != 5 || !all[0].OpenDota || all[1].OpenDota {
		t.Fatalf("merged order: %+v", all)
	}
	od := all[0]
	if !od.Win || od.Team != teamDire || od.HeroName != "npc_dota_hero_axe" || od.Lane != LaneTop || od.TotalGold != 10000 || od.EndTime != 1300 {
		t.Fatalf("history game: %+v", od)
	}
	avg := map[string]MetricAvg{}
	for _, a := range averages(all) {
		avg[a.Key] = a
	}
	// Replay-only metrics ignore the OpenDota row; basic ones use it.
	if avg["stacks"].N != 2 || avg["stacks"].Avg != 3 || avg["kills"].N != 3 || avg["hero_dmg_pm"].N != 2 || avg["gpm"].N != 3 {
		t.Fatalf("averages: %+v", avg)
	}
	if HeroNamer(nil)(99) != "npc_dota_hero_99" {
		t.Error("fallback name")
	}
}
