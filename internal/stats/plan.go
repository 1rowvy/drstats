package stats

import (
	"math"
	"sort"
	"time"

	"github.com/1rowvy/drstats/internal/store"
)

// Weekly plan: 2–3 goals picked from the weakest metrics, tracked game by game.
const (
	PlanKey     = "plan" // settings-table key the plan is stored under
	PlanDays    = 7      // plan length
	PlanGoals   = 3      // goals per plan
	planWindow  = 30     // recent games the goals are picked from
	planRecent  = 15     // games the baseline is the average of
	planMinVals = 10     // values a metric needs to be a candidate
	planSupport = 25     // median LH@10 below this = support: no farm goals
	// Win impact: games a goal's winrate split is measured over, and how
	// many games each side needs before it counts.
	impactWindow = 100
	impactMin    = 5
)

// PlanGoal is one goal of a plan.
type PlanGoal struct {
	Key          string  `json:"key"`
	HigherBetter bool    `json:"higher_better"`
	Base         float64 `json:"base"`   // your average when the plan was made
	Target       float64 `json:"target"` // halfway to your best games
	// Winrate of your games that met / missed the target (nil when either
	// side has too few games): why this goal matters.
	WinMet  *float64 `json:"win_met"`
	WinMiss *float64 `json:"win_miss"`
	NMet    int      `json:"n_met"`
	NMiss   int      `json:"n_miss"`
}

// Plan is the stored weekly plan.
type Plan struct {
	Start int64      `json:"start"` // unix
	End   int64      `json:"end"`
	Goals []PlanGoal `json:"goals"`
}

// GoalGame is one game's value for a goal.
type GoalGame struct {
	MatchID int64   `json:"match_id"`
	Value   float64 `json:"value"`
	Met     bool    `json:"met"`
}

// GoalProgress is a goal with the games played since the plan started.
type GoalProgress struct {
	PlanGoal
	Current *float64   `json:"current"` // average since the start, nil without games
	Hits    int        `json:"hits"`    // games that met the target
	Games   []GoalGame `json:"games"`   // oldest first
}

// PlanView is what the dashboard shows.
type PlanView struct {
	Plan    *Plan          `json:"plan"` // nil when there is none
	Goals   []GoalProgress `json:"goals"`
	Games   int            `json:"games"`   // games since the start
	Expired bool           `json:"expired"` // the week is over
	Unknown bool           `json:"unknown"` // no Steam ID
	// Enough is false when there are too few parsed games to build a plan.
	Enough bool `json:"enough"`
}

// gameX are per-game values that do not come from the games table.
type gameX struct {
	farm  *farmGame
	avoid int // avoidable deaths
}

// extras maps match id to its extra values (parsed replays only).
type extras map[int64]gameX

func (e extras) of(g *Game) (gameX, bool) {
	x, ok := e[g.MatchID]
	return x, ok && !g.OpenDota
}

// planMetric is a per-game value a goal (or a match reason) can be about.
type planMetric struct {
	key          string
	higherBetter bool
	core         bool // only for cores (farm)
	goal         bool // can be a plan goal
	weight       float64
	value        func(g *Game, x gameX) (float64, bool)
}

var planMetrics = []planMetric{
	{"lh10", true, true, true, 1.2, func(g *Game, _ gameX) (float64, bool) { return intp(g.LH10) }},
	{"deaths", false, false, true, 1.2, func(g *Game, _ gameX) (float64, bool) { return float64(g.Deaths), true }},
	{"deaths10", false, false, true, 1, func(g *Game, _ gameX) (float64, bool) { return float64(g.Deaths10), true }},
	{"avoid", false, false, true, 1.3, func(_ *Game, x gameX) (float64, bool) { return float64(x.avoid), true }},
	{"kp", true, false, true, 1, func(g *Game, _ gameX) (float64, bool) {
		if g.TeamKills == 0 {
			return 0, false
		}
		return float64(g.Kills+g.Assists) / float64(g.TeamKills), true
	}},
	{"lane_nw", true, false, true, 1, func(g *Game, _ gameX) (float64, bool) {
		if g.Opp == nil || g.NW10 == nil {
			return 0, false
		}
		return float64(*g.NW10 - g.Opp.NW10), true
	}},
	{"mid_idle", false, false, true, 1.1, func(_ *Game, x gameX) (float64, bool) {
		if x.farm == nil || !x.farm.phases[1].ok {
			return 0, false
		}
		return x.farm.phases[1].idlePct(), true
	}},
	{"farm_gpm", true, true, true, 1, func(_ *Game, x gameX) (float64, bool) {
		if x.farm == nil || x.farm.farmGPM <= 0 {
			return 0, false
		}
		return x.farm.farmGPM, true
	}},
	// Match reasons only.
	{"gpm", true, false, false, 1, func(g *Game, _ gameX) (float64, bool) { return perMin(g.TotalGold, g) }},
	{"xpm", true, false, false, 0.8, func(g *Game, _ gameX) (float64, bool) { return perMin(g.TotalXP, g) }},
	{"hero_dmg_pm", true, false, false, 0.9, func(g *Game, _ gameX) (float64, bool) { return perMin(g.HeroDamage, g) }},
}

func metricByKey(key string) (planMetric, bool) {
	for _, m := range planMetrics {
		if m.key == key {
			return m, true
		}
	}
	return planMetric{}, false
}

// winSplit is the winrate of the games on the good side of a threshold vs
// the rest (the last impactWindow parsed games with a value).
func winSplit(games []Game, ex extras, m planMetric, threshold float64) (met, miss Bucket) {
	var idx []int
	for i := len(games) - 1; i >= 0 && len(idx) < impactWindow; i-- {
		if _, ok := ex.of(&games[i]); ok {
			idx = append(idx, i)
		}
	}
	for _, i := range idx {
		g := &games[i]
		x, _ := ex.of(g)
		v, ok := m.value(g, x)
		if !ok {
			continue
		}
		good := v >= threshold
		if !m.higherBetter {
			good = v <= threshold
		}
		b := &miss
		if good {
			b = &met
		}
		b.Games++
		if g.Win {
			b.Wins++
		}
	}
	finishBucket(&met)
	finishBucket(&miss)
	return met, miss
}

// impact is the winrate gap met − miss, ok when both sides have enough games.
func impact(met, miss Bucket) (float64, bool) {
	if met.Games < impactMin || miss.Games < impactMin {
		return 0, false
	}
	return met.Winrate - miss.Winrate, true
}

// planGames loads the account's parsed games (oldest first) with their extras.
func planGames(st *store.Store, accountID uint32, threshold float64) ([]Game, extras, error) {
	games, err := loadGames(st, accountID)
	if err != nil {
		return nil, nil, err
	}
	ex, err := loadExtras(st, accountID, games, threshold)
	return games, ex, err
}

// loadExtras computes farm numbers and avoidable deaths of the parsed games.
func loadExtras(st *store.Store, accountID uint32, games []Game, threshold float64) (extras, error) {
	series, err := st.FarmSeries(accountID)
	if err != nil {
		return nil, err
	}
	scenes, err := st.DeathScenes(accountID)
	if err != nil {
		return nil, err
	}
	return buildExtrasMap(games, farmGames(games, series, threshold), avoidableCounts(classifyScenes(scenes))), nil
}

func buildExtrasMap(games []Game, fgs map[int64]farmGame, avoid map[int64]int) extras {
	ex := extras{}
	for i := range games {
		g := &games[i]
		if g.OpenDota {
			continue
		}
		x := gameX{avoid: avoid[g.MatchID]}
		if fg, ok := fgs[g.MatchID]; ok {
			x.farm = &fg
		}
		ex[g.MatchID] = x
	}
	return ex
}

// NewPlan picks the goals from the recent games; nil when there are too few.
func NewPlan(all []Game, ex extras, now time.Time) *Plan {
	var games []Game
	for i := range all {
		if _, ok := ex.of(&all[i]); ok {
			games = append(games, all[i])
		}
	}
	if len(games) > planWindow {
		games = games[len(games)-planWindow:]
	}
	var lh []float64
	for i := range games {
		if v, ok := intp(games[i].LH10); ok {
			lh = append(lh, v)
		}
	}
	support := len(lh) > 0 && median(lh) < planSupport
	type cand struct {
		goal  PlanGoal
		score float64
	}
	var cands []cand
	for _, m := range planMetrics {
		if !m.goal || m.core && support {
			continue
		}
		var vals []float64
		for i := range games {
			x, _ := ex.of(&games[i])
			if v, ok := m.value(&games[i], x); ok {
				vals = append(vals, v)
			}
		}
		if len(vals) < planMinVals {
			continue
		}
		recent := vals
		if len(recent) > planRecent {
			recent = recent[len(recent)-planRecent:]
		}
		base := meanOf(recent)
		// "Your best games": the best quarter of the window.
		sorted := append([]float64(nil), vals...)
		sort.Float64s(sorted)
		q := max(1, len(sorted)/4)
		best := meanOf(sorted[:q])
		if m.higherBetter {
			best = meanOf(sorted[len(sorted)-q:])
		}
		gap := best - base
		if !m.higherBetter {
			gap = -gap
		}
		if gap <= 0 {
			continue
		}
		scale := math.Max(math.Abs(base), math.Abs(best))
		if scale == 0 {
			continue
		}
		goal := PlanGoal{Key: m.key, HigherBetter: m.higherBetter, Base: base, Target: base + (best-base)/2}
		score := gap / scale * m.weight
		// Prefer goals that actually decide your games.
		met, miss := winSplit(all, ex, m, goal.Target)
		goal.NMet, goal.NMiss = met.Games, miss.Games
		if d, ok := impact(met, miss); ok {
			goal.WinMet, goal.WinMiss = &met.Winrate, &miss.Winrate
			score *= math.Max(0.25, math.Min(2.5, 1+4*d))
		}
		cands = append(cands, cand{goal: goal, score: score})
	}
	if len(cands) == 0 {
		return nil
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	p := &Plan{Start: now.Unix(), End: now.AddDate(0, 0, PlanDays).Unix(), Goals: []PlanGoal{}}
	for _, c := range cands {
		if len(p.Goals) == PlanGoals {
			break
		}
		// One deaths goal is enough.
		if c.goal.Key == "deaths10" && hasGoal(p, "deaths") || c.goal.Key == "deaths" && hasGoal(p, "deaths10") {
			continue
		}
		p.Goals = append(p.Goals, c.goal)
	}
	return p
}

func hasGoal(p *Plan, key string) bool {
	for _, g := range p.Goals {
		if g.Key == key {
			return true
		}
	}
	return false
}

// PlanProgress tracks a plan over the games played since it started.
func PlanProgress(p *Plan, games []Game, ex extras, now time.Time) PlanView {
	v := PlanView{Plan: p, Goals: []GoalProgress{}, Enough: true}
	if p == nil {
		return v
	}
	v.Expired = now.Unix() >= p.End
	for _, goal := range p.Goals {
		gp := GoalProgress{PlanGoal: goal, Games: []GoalGame{}}
		m, ok := metricByKey(goal.Key)
		var sum float64
		for i := range games {
			g := &games[i]
			x, has := ex.of(g)
			if !has || g.EndTime < p.Start || g.EndTime >= p.End {
				continue
			}
			if !ok {
				continue
			}
			val, okv := m.value(g, x)
			if !okv {
				continue
			}
			met := val >= goal.Target
			if !goal.HigherBetter {
				met = val <= goal.Target
			}
			if met {
				gp.Hits++
			}
			sum += val
			gp.Games = append(gp.Games, GoalGame{MatchID: g.MatchID, Value: val, Met: met})
		}
		if n := len(gp.Games); n > 0 {
			c := sum / float64(n)
			gp.Current = &c
		}
		v.Goals = append(v.Goals, gp)
	}
	for i := range games {
		if !games[i].OpenDota && games[i].EndTime >= p.Start && games[i].EndTime < p.End {
			v.Games++
		}
	}
	return v
}

// GetPlan returns the stored plan with its progress.
func GetPlan(st *store.Store, accountID uint32, threshold float64, now time.Time) (PlanView, error) {
	if accountID == 0 {
		return PlanView{Goals: []GoalProgress{}, Unknown: true}, nil
	}
	var p Plan
	found, err := st.GetSetting(PlanKey, &p)
	if err != nil {
		return PlanView{}, err
	}
	games, ex, err := planGames(st, accountID, threshold)
	if err != nil {
		return PlanView{}, err
	}
	if !found || len(p.Goals) == 0 {
		v := PlanProgress(nil, games, ex, now)
		v.Enough = NewPlan(games, ex, now) != nil
		return v, nil
	}
	return PlanProgress(&p, games, ex, now), nil
}

// MakePlan builds and stores a new plan from the recent games.
func MakePlan(st *store.Store, accountID uint32, threshold float64, now time.Time) (PlanView, error) {
	if accountID == 0 {
		return PlanView{Goals: []GoalProgress{}, Unknown: true}, nil
	}
	games, ex, err := planGames(st, accountID, threshold)
	if err != nil {
		return PlanView{}, err
	}
	p := NewPlan(games, ex, now)
	if p == nil {
		return PlanView{Goals: []GoalProgress{}, Enough: false}, nil
	}
	if err := st.SetSetting(PlanKey, p); err != nil {
		return PlanView{}, err
	}
	return PlanProgress(p, games, ex, now), nil
}

func median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	return s[len(s)/2]
}

func meanOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
