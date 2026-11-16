package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

func TestFarmGameAndStats(t *testing.T) {
	// 30-minute game sampled every 5 s: farming 100 creep gold/min in the lane,
	// then in mid game a 2-minute idle stretch (11:00–13:00) and 60 s dead (20:00–21:00).
	var ss []parser.Sample
	creep, neutral, lh := 0, 0, 0
	for tt := 0.0; tt <= 1800; tt += 5 {
		idle := tt > 660 && tt <= 780
		dead := tt > 1200 && tt <= 1260
		if !idle && !dead {
			lh++
			if tt < 600 {
				creep += 100 * 5 / 60
			} else {
				neutral += 200 * 5 / 60
			}
		}
		ss = append(ss, parser.Sample{T: tt, Alive: !dead, LastHits: lh, GoldCreep: creep, GoldNeutral: neutral})
	}
	fg, ok := buildFarmGame(ss, 1800, 20)
	if !ok {
		t.Fatal("no farm game")
	}
	lane, mid := fg.phases[0], fg.phases[1]
	if lane.idle != 0 || lane.creep < 800 || lane.neutral > 20 {
		t.Errorf("lane %+v", lane)
	}
	if mid.idle < 110 || mid.idle > 125 || mid.dead < 55 || mid.dead > 65 {
		t.Errorf("mid idle %.0f dead %.0f", mid.idle, mid.dead)
	}
	g := Game{GameRow: store.GameRow{MatchID: 1, Duration: 1800}, Win: true}
	fs := buildFarmStats([]Game{g}, map[int64]farmGame{1: fg}, 20)
	if fs.Games != 1 || fs.Phases[1].NeutralGPM < 100 || fs.Phases[1].IdlePct <= 0 || fs.IdlePerGame < 110 {
		t.Errorf("farm stats %+v", fs)
	}
}
