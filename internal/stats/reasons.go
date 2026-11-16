package stats

import (
	"math"
	"sort"
)

// Match reasons: what set this game apart from the player's usual games —
// for a loss against their wins (why it was lost), for a win against all
// their games (what won it). Each gap is weighted by how much that metric
// decides the player's games.
const (
	reasonMinRef    = 5   // reference games needed
	reasonHeroRef   = 8   // on-hero reference from this many games
	reasonMinZ      = 0.6 // standardized gap that counts as a reason
	reasonMaxMain   = 3   // reasons for the result
	reasonMaxOther  = 2   // the other side (did well in a loss / weak in a win)
	reasonMinSpread = 1e-9
)

// Reason is one metric where the game stood out.
type Reason struct {
	Key          string  `json:"key"`
	HigherBetter bool    `json:"higher_better"`
	Value        float64 `json:"value"` // this game
	Ref          float64 `json:"ref"`   // median of the reference games
	Z            float64 `json:"z"`     // gap in spreads, > 0 better than usual
	// Winrate of your games on the good / bad side of your usual value
	// (nil when either side has too few games).
	WinGood *float64 `json:"win_good"`
	WinBad  *float64 `json:"win_bad"`
	Score   float64  `json:"score"`
}

// MatchReasons explains a game against the player's own games.
type MatchReasons struct {
	Win    bool `json:"win"`
	Basis  int  `json:"basis"`   // reference games
	OnHero bool `json:"on_hero"` // reference on the same hero
	// RefWins: the reference is your wins (a lost game), else all games.
	RefWins bool     `json:"ref_wins"`
	Main    []Reason `json:"main"`  // why lost / what won it, most important first
	Other   []Reason `json:"other"` // did well despite the loss / weak spots of a win
}

// buildReasons compares the game with id matchID against the others.
// nil when the game is unknown or there are too few reference games.
func buildReasons(games []Game, ex extras, matchID int64) *MatchReasons {
	gi := -1
	for i := range games {
		if games[i].MatchID == matchID {
			gi = i
		}
	}
	if gi < 0 {
		return nil
	}
	g := &games[gi]
	x, ok := ex.of(g)
	if !ok {
		return nil
	}
	// Reference: parsed games other than this one; for a loss only wins.
	pick := func(sameHero bool) []int {
		var out []int
		for i := range games {
			o := &games[i]
			if i == gi || (sameHero && o.HeroID != g.HeroID) || (!g.Win && !o.Win) {
				continue
			}
			if _, ok := ex.of(o); ok {
				out = append(out, i)
			}
		}
		return out
	}
	r := &MatchReasons{Win: g.Win, RefWins: !g.Win, Main: []Reason{}, Other: []Reason{}}
	ref := pick(true)
	r.OnHero = len(ref) >= reasonHeroRef
	if !r.OnHero {
		ref = pick(false)
	}
	if len(ref) < reasonMinRef {
		return nil
	}
	r.Basis = len(ref)
	supportGame := g.LH10 != nil && *g.LH10 < planSupport
	var good, bad []Reason
	for _, m := range planMetrics {
		if m.core && supportGame {
			continue
		}
		v, ok := m.value(g, x)
		if !ok {
			continue
		}
		var vals []float64
		for _, i := range ref {
			ox, _ := ex.of(&games[i])
			if o, ok := m.value(&games[i], ox); ok {
				vals = append(vals, o)
			}
		}
		if len(vals) < reasonMinRef {
			continue
		}
		med, spread := robustSpread(vals)
		if spread <= reasonMinSpread {
			continue
		}
		z := (v - med) / spread
		if !m.higherBetter {
			z = -z
		}
		rs := Reason{Key: m.key, HigherBetter: m.higherBetter, Value: v, Ref: med, Z: z}
		// How much this metric decides your games: winrate split at your
		// usual value over all your parsed games.
		var all []float64
		for i := range games {
			ox, ok := ex.of(&games[i])
			if !ok {
				continue
			}
			if o, ok := m.value(&games[i], ox); ok {
				all = append(all, o)
			}
		}
		weight := m.weight
		if len(all) > 0 {
			met, miss := winSplit(games, ex, m, median(all))
			if d, ok := impact(met, miss); ok {
				rs.WinGood, rs.WinBad = &met.Winrate, &miss.Winrate
				weight *= math.Max(0.3, math.Min(2.5, 1+4*d))
			}
		}
		rs.Score = math.Abs(z) * weight
		switch {
		case z <= -reasonMinZ:
			bad = append(bad, rs)
		case z >= reasonMinZ:
			good = append(good, rs)
		}
	}
	byScore := func(xs []Reason) {
		sort.Slice(xs, func(i, j int) bool { return xs[i].Score > xs[j].Score })
	}
	byScore(good)
	byScore(bad)
	// Deaths, deaths before 10:00 and avoidable deaths tell the same story:
	// keep the strongest one.
	bad = dedupeDeaths(bad)
	good = dedupeDeaths(good)
	main, other := bad, good
	if g.Win {
		main, other = good, bad
	}
	r.Main = append(r.Main, main[:min(len(main), reasonMaxMain)]...)
	r.Other = append(r.Other, other[:min(len(other), reasonMaxOther)]...)
	return r
}

func dedupeDeaths(xs []Reason) []Reason {
	out := xs[:0:0]
	seen := false
	for _, r := range xs {
		if r.Key == "deaths" || r.Key == "deaths10" || r.Key == "avoid" {
			if seen {
				continue
			}
			seen = true
		}
		out = append(out, r)
	}
	return out
}

// robustSpread is the median and a spread estimate (IQR / 1.35, at least
// half the standard deviation) that small samples of counts do not zero out.
func robustSpread(vals []float64) (med, spread float64) {
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	med = quantile(s, 0.5)
	iqr := quantile(s, 0.75) - quantile(s, 0.25)
	mean := meanOf(s)
	var ss float64
	for _, v := range s {
		ss += (v - mean) * (v - mean)
	}
	sd := math.Sqrt(ss / float64(len(s)))
	return med, math.Max(iqr/1.35, sd/2)
}

// quantile of sorted values, linear interpolation.
func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := min(lo+1, len(sorted)-1)
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}
