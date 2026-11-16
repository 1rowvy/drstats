package stats

import (
	"sort"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

// Lane outcomes against the lane opponent at 10:00.
const (
	LaneWon  = "won"
	LaneEven = "even"
	LaneLost = "lost"
)

// LaneScoreEdge is the |score| from which a lane counts as won / lost.
const LaneScoreEdge = 500

// LaneScore rates the lane at 10:00: net worth difference plus half the
// experience difference (levels matter, but gold decides more).
func LaneScore(nw, oppNW, xp, oppXP int) float64 {
	return float64(nw-oppNW) + float64(xp-oppXP)/2
}

// LaneOutcome turns a LaneScore into won / even / lost.
func LaneOutcome(score float64) string {
	switch {
	case score >= LaneScoreEdge:
		return LaneWon
	case score <= -LaneScoreEdge:
		return LaneLost
	}
	return LaneEven
}

// LaneOpp is the lane opponent of a game with their values at 10:00.
type LaneOpp struct {
	HeroName string `json:"hero_name"`
	NW10     int    `json:"nw10"`
	XP10     int    `json:"xp10"`
	LH10     int    `json:"lh10"`
}

// attachLaneOpps fills Game.Opp / LaneScore / LaneResult from the stored rivals.
func attachLaneOpps(games []Game, rivals map[int64][]store.Rival) {
	for i := range games {
		g := &games[i]
		if g.OpenDota || g.NW10 == nil || g.XP10 == nil {
			continue
		}
		rs := rivals[g.MatchID]
		cands := map[int][]parser.Sample{}
		for _, r := range rs {
			cands[r.PlayerID] = r.Samples
		}
		pid := closestRival(g.LaneEarly, cands)
		if pid < 0 {
			continue
		}
		for _, r := range rs {
			if r.PlayerID != pid {
				continue
			}
			s, ok := sampleAt(r.Samples, 600)
			if !ok {
				break
			}
			g.Opp = &LaneOpp{HeroName: r.HeroName, NW10: s.NetWorth, XP10: s.TotalXP, LH10: s.LastHits}
			g.LaneScore = LaneScore(*g.NW10, s.NetWorth, *g.XP10, s.TotalXP)
			g.LaneResult = LaneOutcome(g.LaneScore)
		}
	}
}

// LaneMatchup is the lane record against one opponent hero.
type LaneMatchup struct {
	HeroName string  `json:"hero_name"`
	Games    int     `json:"games"`
	Won      int     `json:"won"`
	Lost     int     `json:"lost"`
	AvgScore float64 `json:"avg_score"` // average LaneScore
	AvgNW    float64 `json:"avg_nw"`    // average net worth difference at 10:00
}

// LaneStats summarise lanes against the lane opponent over the filtered games.
type LaneStats struct {
	Games int `json:"games"` // games with a known opponent
	// Game results after a won / even / lost lane (Key: won / even / lost).
	Won  Bucket `json:"won"`
	Even Bucket `json:"even"`
	Lost Bucket `json:"lost"`
	// AvgNW is the average net worth difference at 10:00.
	AvgNW float64 `json:"avg_nw"`
	// Matchups by opponent hero, hardest first (lowest AvgScore).
	Matchups []LaneMatchup `json:"matchups"`
}

func laneStats(games []Game) LaneStats {
	ls := LaneStats{Won: Bucket{Key: LaneWon}, Even: Bucket{Key: LaneEven}, Lost: Bucket{Key: LaneLost}, Matchups: []LaneMatchup{}}
	by := map[string]*LaneMatchup{}
	var nwSum float64
	for i := range games {
		g := &games[i]
		if g.Opp == nil {
			continue
		}
		ls.Games++
		nw := float64(*g.NW10 - g.Opp.NW10)
		nwSum += nw
		switch g.LaneResult {
		case LaneWon:
			addBucket(&ls.Won, g)
		case LaneLost:
			addBucket(&ls.Lost, g)
		default:
			addBucket(&ls.Even, g)
		}
		m := by[g.Opp.HeroName]
		if m == nil {
			m = &LaneMatchup{HeroName: g.Opp.HeroName}
			by[g.Opp.HeroName] = m
		}
		m.Games++
		m.AvgScore += g.LaneScore
		m.AvgNW += nw
		switch g.LaneResult {
		case LaneWon:
			m.Won++
		case LaneLost:
			m.Lost++
		}
	}
	finishBucket(&ls.Won)
	finishBucket(&ls.Even)
	finishBucket(&ls.Lost)
	if ls.Games > 0 {
		ls.AvgNW = nwSum / float64(ls.Games)
	}
	for _, m := range by {
		m.AvgScore /= float64(m.Games)
		m.AvgNW /= float64(m.Games)
		ls.Matchups = append(ls.Matchups, *m)
	}
	sort.Slice(ls.Matchups, func(i, j int) bool {
		a, b := ls.Matchups[i], ls.Matchups[j]
		if a.AvgScore != b.AvgScore {
			return a.AvgScore < b.AvgScore
		}
		return a.HeroName < b.HeroName
	})
	return ls
}
