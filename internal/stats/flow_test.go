package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/store"
)

func TestBuildFlow(t *testing.T) {
	// 40-minute game: lead rises to +8k at 20:00, then collapses and flips at 30:00; lost.
	f := &store.GameFlow{}
	for m := 0; m <= 40; m++ {
		lead := 0
		switch {
		case m <= 20:
			lead = m * 400
		default:
			lead = 8000 - (m-20)*800
		}
		f.Lead = append(f.Lead, store.LeadPoint{T: float64(m * 60), Lead: lead})
		f.Samples = append(f.Samples, store.FlowSample{T: float64(m * 60), TotalGold: m * 500, TotalXP: m * 600, LastHits: m * 5})
	}
	// Two enemy kills in the lane (one yours), you die twice during the collapse.
	f.Deaths = []store.FlowDeath{
		{T: 300, Credit: true}, {T: 400},
		{T: 1400, Mine: true, Ally: true}, {T: 1600, Mine: true, Ally: true}, {T: 1650, Credit: true},
	}
	g := Game{GameRow: store.GameRow{MatchID: 1, Duration: 2400, TotalGold: 20000}, Win: false}
	short := Game{GameRow: store.GameRow{MatchID: 2, Duration: 1200}, Win: true, OpenDota: true}
	fs := buildFlow([]Game{g, short}, map[int64]*store.GameFlow{1: f})

	if fs.Games != 1 || len(fs.Phases) != 3 {
		t.Fatalf("games %d phases %d", fs.Games, len(fs.Phases))
	}
	lane := fs.Phases[0]
	if lane.Games != 1 || lane.GPM != 500 || lane.KP != 0.5 || lane.LeadDelta != 4000 {
		t.Errorf("lane %+v", lane)
	}
	late := fs.Phases[2]
	if late.Games != 1 || late.LeadDelta != -12000 || late.Gained != 0 {
		t.Errorf("late %+v", late)
	}
	if len(fs.Thrown) != 1 || fs.Ahead.Games != 1 || fs.Ahead.Wins != 0 {
		t.Fatalf("thrown %+v ahead %+v", fs.Thrown, fs.Ahead)
	}
	th := fs.Thrown[0]
	if th.Peak != 8000 || th.PeakT != 1200 || th.FlipT != 1800 || th.Deaths != 2 || th.KP != 1 {
		t.Errorf("throw %+v", th)
	}
	// Length buckets count the history game too.
	if fs.ByLength[0].Games != 1 || fs.ByLength[2].Games != 1 {
		t.Errorf("by length %+v", fs.ByLength)
	}
}
