package stats

import (
	"math"

	"github.com/1rowvy/drstats/internal/parser"
)

// Map regions. The geometry is a coarse approximation of the 7.3x+ map in
// world units (Radiant base bottom-left, Dire top-right): good enough for
// lane detection and time shares, not for exact zone borders.
const (
	LaneTop = "top"
	LaneMid = "mid"
	LaneBot = "bot"
	// LaneJungle / LaneRoam: no lane held the player for most of the laning stage.
	LaneJungle = "jungle"
	LaneRoam   = "roam"
)

var (
	radiantBase = [2]float64{-6700, -6200}
	direBase    = [2]float64{6500, 6000}
)

const baseRadius = 3300.0

// inBase reports whether a point is inside the given team's base.
func inBase(team int, x, y float64) bool {
	c := radiantBase
	if team == parser.TeamDire {
		c = direBase
	}
	return math.Hypot(x-c[0], y-c[1]) < baseRadius
}

// laneAt returns the lane band a point lies on ("" if none).
func laneAt(x, y float64) string {
	switch {
	case (x < -5300 && y > -3500) || (y > 4700 && x < 3500):
		return LaneTop
	case (y < -5300 && x > -3500) || (x > 4900 && y < 3500):
		return LaneBot
	case math.Abs(x-y) < 1700 && math.Abs(x) < 5200:
		return LaneMid
	}
	return ""
}

// inJungleArea: not on a lane band and not in a base.
func inJungleArea(x, y float64) bool {
	return laneAt(x, y) == "" && !inBase(parser.TeamRadiant, x, y) && !inBase(parser.TeamDire, x, y)
}

// enemyHalf reports whether a point is on the enemy side of the river
// (the river runs along x + y = 0).
func enemyHalf(team int, x, y float64) bool {
	if team == parser.TeamDire {
		return x+y < 0
	}
	return x+y > 0
}

// DetectLane picks the lane where the player spent most of the laning stage
// (1:00–10:00, alive samples). Under 40% on any lane means jungle (mostly in
// jungle areas) or roam.
func DetectLane(samples []parser.Sample) string {
	counts := map[string]int{}
	jungle, n := 0, 0
	for _, s := range samples {
		if s.T < laneFrom || s.T > laneTo || !s.Alive {
			continue
		}
		n++
		if l := laneAt(s.X, s.Y); l != "" {
			counts[l]++
		} else if inJungleArea(s.X, s.Y) {
			jungle++
		}
	}
	if n == 0 {
		return ""
	}
	best, bestN := "", 0
	for _, l := range []string{LaneTop, LaneMid, LaneBot} {
		if counts[l] > bestN {
			best, bestN = l, counts[l]
		}
	}
	if float64(bestN) >= 0.4*float64(n) {
		return best
	}
	if float64(jungle) >= 0.4*float64(n) {
		return LaneJungle
	}
	return LaneRoam
}

// TimeShare is how the player's alive time splits between map zones (fractions, sum 1).
type TimeShare struct {
	Base   float64 `json:"base"`   // own base
	Lane   float64 `json:"lane"`   // own laning-stage lane, own half
	Jungle float64 `json:"jungle"` // own half outside the own lane (jungle, river, other lanes)
	Enemy  float64 `json:"enemy"`  // enemy half (incl. enemy base)
}

// ComputeTimeShare classifies alive samples after the horn.
func ComputeTimeShare(samples []parser.Sample, team int, lane string) TimeShare {
	var ts TimeShare
	n := 0
	for _, s := range samples {
		if !s.Alive || s.T < 0 {
			continue
		}
		n++
		switch {
		case inBase(team, s.X, s.Y):
			ts.Base++
		case enemyHalf(team, s.X, s.Y):
			ts.Enemy++
		case lane != "" && laneAt(s.X, s.Y) == lane:
			ts.Lane++
		default:
			ts.Jungle++
		}
	}
	if n > 0 {
		f := float64(n)
		ts.Base, ts.Lane, ts.Jungle, ts.Enemy = ts.Base/f, ts.Lane/f, ts.Jungle/f, ts.Enemy/f
	}
	return ts
}

// River polyline (world units), top-left to bottom-right; same shape as the
// schematic map in the UI.
var river = [][2]float64{{-7400, 4600}, {-3000, 1600}, {-600, -400}, {2000, -2600}, {5000, -7400}}

const riverHalfWidth = 750.0

func distToSegment(x, y float64, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	t := ((x-a[0])*dx + (y-a[1])*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(x-(a[0]+t*dx), y-(a[1]+t*dy))
}

func inRiver(x, y float64) bool {
	for i := 1; i < len(river); i++ {
		if distToSegment(x, y, river[i-1], river[i]) < riverHalfWidth {
			return true
		}
	}
	return false
}

// side returns which team's half of the map a point is on ("radiant" / "dire").
func side(x, y float64) string {
	if x+y < -800 {
		return "radiant"
	}
	return "dire"
}

// Region names a map zone as a key the UI turns into a label:
// "base:<side>", "river:<top|mid|bot>", "lane:<top|mid|bot>:<side>", "jungle:<side>".
// Coarse geometry, meant for human-readable death/idle locations.
func Region(x, y float64) string {
	switch {
	case inBase(parser.TeamRadiant, x, y):
		return "base:radiant"
	case inBase(parser.TeamDire, x, y):
		return "base:dire"
	case inRiver(x, y):
		part := LaneMid
		if x < -2600 {
			part = LaneTop
		} else if x > 1800 {
			part = LaneBot
		}
		return "river:" + part
	}
	if l := laneAt(x, y); l != "" {
		return "lane:" + l + ":" + side(x, y)
	}
	return "jungle:" + side(x, y)
}
