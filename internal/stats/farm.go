package stats

import (
	"math"
	"sort"

	"github.com/1rowvy/drstats/internal/parser"
)

// FarmPhase is farm efficiency in one game phase, averaged over games.
type FarmPhase struct {
	Key        string  `json:"key"` // lane / mid / late
	Games      int     `json:"games"`
	FarmGPM    float64 `json:"farm_gpm"`    // creep + neutral gold per minute
	CreepGPM   float64 `json:"creep_gpm"`   // lane creeps
	NeutralGPM float64 `json:"neutral_gpm"` // jungle
	IdlePct    float64 `json:"idle_pct"`    // share of alive time without farm or fight
	DeadPct    float64 `json:"dead_pct"`    // share of time dead
}

// IdleZone is where idle time is spent.
type IdleZone struct {
	Region string  `json:"region"`
	Sec    float64 `json:"sec"` // average per game
}

// FarmStats summarise farm efficiency over the filtered games with a replay.
type FarmStats struct {
	Games       int         `json:"games"`
	Threshold   float64     `json:"threshold"` // idle threshold, seconds
	Phases      []FarmPhase `json:"phases"`
	IdlePerGame float64     `json:"idle_per_game"` // seconds
	Zones       []IdleZone  `json:"zones"`         // biggest first
	// Winrate of the games with less / more idle share than the median.
	LowIdle  Bucket `json:"low_idle"`
	HighIdle Bucket `json:"high_idle"`
	// Farm GPM of the last HabitRecent games against the earlier ones (0 when too few).
	RecentFarmGPM float64 `json:"recent_farm_gpm"`
	BeforeFarmGPM float64 `json:"before_farm_gpm"`
}

// farmGame are one game's farm numbers.
type farmGame struct {
	phases  [3]farmPhaseVals
	idle    float64 // seconds, whole game
	idlePct float64 // whole game, of alive time
	farmGPM float64 // whole game
	zones   map[string]float64
}

type farmPhaseVals struct {
	ok                   bool
	creep, neutral, mins float64
	idle, alive, dead    float64
}

func (v farmPhaseVals) idlePct() float64 {
	if v.alive <= 0 {
		return 0
	}
	return v.idle / v.alive
}

// buildFarmGame computes one game's farm numbers from the full sample series.
func buildFarmGame(ss []parser.Sample, end, threshold float64) (farmGame, bool) {
	var fg farmGame
	fg.zones = map[string]float64{}
	if len(ss) < 2 {
		return fg, false
	}
	idle := IdlePeriods(ss, threshold)
	// Idle seconds in [from, to).
	idleIn := func(from, to float64) float64 {
		var sum float64
		for _, p := range idle {
			sum += math.Max(0, math.Min(p.To, to)-math.Max(p.From, from))
		}
		return sum
	}
	for _, p := range idle {
		fg.idle += p.To - p.From
		fg.zones[p.Region] += p.To - p.From
	}
	var aliveAll float64
	for k, b := range phaseBounds {
		from, to := b[0], math.Min(b[1], end)
		if to-from < PhaseMinLen {
			continue
		}
		a, okA := sampleAtOrBefore(ss, from)
		z, okZ := sampleAtOrBefore(ss, to)
		if !okA || !okZ || z.T-a.T < 60 {
			continue
		}
		v := farmPhaseVals{ok: true, mins: (z.T - a.T) / 60,
			creep: float64(z.GoldCreep - a.GoldCreep), neutral: float64(z.GoldNeutral - a.GoldNeutral)}
		for i := 1; i < len(ss); i++ {
			t0, t1 := math.Max(ss[i-1].T, from), math.Min(ss[i].T, to)
			if t1 <= t0 {
				continue
			}
			if ss[i].Alive && ss[i-1].Alive {
				v.alive += t1 - t0
			} else {
				v.dead += t1 - t0
			}
		}
		v.idle = idleIn(from, to)
		aliveAll += v.alive
		fg.phases[k] = v
	}
	last := ss[len(ss)-1]
	first, _ := sampleAtOrBefore(ss, 0)
	if mins := (last.T - first.T) / 60; mins > 0 {
		fg.farmGPM = float64(last.GoldCreep+last.GoldNeutral-first.GoldCreep-first.GoldNeutral) / mins
	}
	if aliveAll > 0 {
		fg.idlePct = fg.idle / aliveAll
	}
	return fg, fg.phases[0].ok
}

func sampleAtOrBefore(ss []parser.Sample, t float64) (parser.Sample, bool) {
	i := sort.Search(len(ss), func(i int) bool { return ss[i].T > t })
	if i == 0 {
		return parser.Sample{}, false
	}
	return ss[i-1], true
}

// farmGames computes the farm numbers of every game with a series.
func farmGames(games []Game, series map[int64][]parser.Sample, threshold float64) map[int64]farmGame {
	out := map[int64]farmGame{}
	for i := range games {
		g := &games[i]
		if g.OpenDota {
			continue
		}
		if fg, ok := buildFarmGame(series[g.MatchID], g.Duration, threshold); ok {
			out[g.MatchID] = fg
		}
	}
	return out
}

func buildFarmStats(games []Game, fgs map[int64]farmGame, threshold float64) FarmStats {
	fs := FarmStats{Threshold: threshold, Phases: []FarmPhase{}, Zones: []IdleZone{},
		LowIdle: Bucket{Key: "low"}, HighIdle: Bucket{Key: "high"}}
	phases := make([]FarmPhase, len(phaseKeys))
	for k := range phases {
		phases[k].Key = phaseKeys[k]
	}
	zones := map[string]float64{}
	var with []*Game
	var pcts []float64
	for i := range games {
		g := &games[i]
		fg, ok := fgs[g.MatchID]
		if !ok {
			continue
		}
		fs.Games++
		fs.IdlePerGame += fg.idle
		for r, s := range fg.zones {
			zones[r] += s
		}
		for k, v := range fg.phases {
			if !v.ok {
				continue
			}
			p := &phases[k]
			p.Games++
			p.FarmGPM += (v.creep + v.neutral) / v.mins
			p.CreepGPM += v.creep / v.mins
			p.NeutralGPM += v.neutral / v.mins
			p.IdlePct += v.idlePct()
			if tot := v.alive + v.dead; tot > 0 {
				p.DeadPct += v.dead / tot
			}
		}
		with = append(with, g)
		pcts = append(pcts, fg.idlePct)
	}
	if fs.Games == 0 {
		fs.Phases = phases
		return fs
	}
	n := float64(fs.Games)
	fs.IdlePerGame /= n
	for k := range phases {
		p := &phases[k]
		if p.Games > 0 {
			m := float64(p.Games)
			p.FarmGPM /= m
			p.CreepGPM /= m
			p.NeutralGPM /= m
			p.IdlePct /= m
			p.DeadPct /= m
		}
	}
	fs.Phases = phases
	for r, s := range zones {
		if r != "" {
			fs.Zones = append(fs.Zones, IdleZone{Region: r, Sec: s / n})
		}
	}
	sort.Slice(fs.Zones, func(i, j int) bool { return fs.Zones[i].Sec > fs.Zones[j].Sec })
	if len(fs.Zones) > 5 {
		fs.Zones = fs.Zones[:5]
	}
	// Idle vs result: split at the median idle share.
	sorted := append([]float64(nil), pcts...)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	for i, g := range with {
		if pcts[i] < median {
			addBucket(&fs.LowIdle, g)
		} else {
			addBucket(&fs.HighIdle, g)
		}
	}
	finishBucket(&fs.LowIdle)
	finishBucket(&fs.HighIdle)
	if len(with) >= HabitRecent+HabitMinBefore {
		var r, b float64
		for i, g := range with {
			v := fgs[g.MatchID].farmGPM
			if i >= len(with)-HabitRecent {
				r += v
			} else {
				b += v
			}
		}
		fs.RecentFarmGPM = r / HabitRecent
		fs.BeforeFarmGPM = b / float64(len(with)-HabitRecent)
	}
	return fs
}
