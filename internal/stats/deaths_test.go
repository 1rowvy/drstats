package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
)

func TestClassifyDeath(t *testing.T) {
	R, D := parser.TeamRadiant, parser.TeamDire
	teams := map[int]int{0: R, 1: R, 5: D, 6: D}
	at := func(pid int, x, y float64, alive bool) parser.Sample {
		return parser.Sample{PlayerID: pid, X: x, Y: y, Alive: alive, Gold: 2000}
	}
	// Dire jungle (deep for Radiant), alone, caught by two.
	deep := parser.Death{T: 1200, VictimPID: 0, KillerPID: 5, Assists: []int{6}, X: 3000, Y: 2000, HasPos: true}
	c := classifyDeath(deep, R, []parser.Sample{at(0, 3000, 2000, true), at(1, -4000, -4000, true)}, teams, nil)
	if !c.Alone || !c.Deep || c.Enemies != 2 || !c.Avoidable || c.Gold != 2000 {
		t.Fatalf("deep: %+v", c)
	}
	// Same spot with an ally next to it: a fight, not a misplay.
	c = classifyDeath(deep, R, []parser.Sample{at(0, 3000, 2000, true), at(1, 3300, 2100, true)}, teams, nil)
	if c.Alone || c.Avoidable || c.Allies != 1 {
		t.Fatalf("with ally: %+v", c)
	}
	// A dead ally nearby does not help.
	c = classifyDeath(deep, R, []parser.Sample{at(0, 3000, 2000, true), at(1, 3300, 2100, false)}, teams, nil)
	if !c.Alone {
		t.Fatalf("dead ally: %+v", c)
	}
	// Own safe lane at 5:00, alone, no vision, one killer: laning, not avoidable.
	lane := parser.Death{T: 300, VictimPID: 0, KillerPID: 5, X: 3000, Y: -6200, HasPos: true}
	c = classifyDeath(lane, R, []parser.Sample{at(0, 3000, -6200, true), at(1, -4000, 4000, true)}, teams, nil)
	if c.Avoidable {
		t.Fatalf("laning: %+v", c)
	}
	// Own jungle at 20:00, alone, blind: avoidable; with a ward: not.
	jungle := parser.Death{T: 1200, VictimPID: 0, KillerPID: 5, X: 1500, Y: -3500, HasPos: true}
	heroes := []parser.Sample{at(0, 1500, -3500, true), at(1, -4000, 4000, true)}
	if c = classifyDeath(jungle, R, heroes, teams, nil); !c.Avoidable || c.Deep {
		t.Fatalf("blind jungle: %+v", c)
	}
	ward := []parser.Ward{{T: 1000, End: 1300, Team: R, X: 1800, Y: -3300}}
	if c = classifyDeath(jungle, R, heroes, teams, ward); c.Avoidable || !c.Vision {
		t.Fatalf("warded jungle: %+v", c)
	}
}
