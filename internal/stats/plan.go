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
)

// PlanGoal is one goal of a plan.
type PlanGoal struct {
	Key          string  `json:"key"`
	HigherBetter bool    `json:"higher_better"`
	Base         float64 `json:"base"`   // your average when the plan was made
	Target       float64 `json:"target"` // halfway to your best games
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

// planMetric is a per-game value a goal can be about.
type planMetric struct {
	key          string
	higherBetter bool
	core         bool // only for cores (farm)
	weight       float64
	value        func(g *Game, fg *farmGame) (float64, bool)
}

var planMetrics = []planMetric{
	{"lh10", true, true, 1.2, func(g *Game, _ *farmGame) (float64, bool) { return intp(g.LH10) }},
	{"deaths", false, false, 1.2, func(g *Game, _ *farmGame) (float64, bool) { return float64(g.Deaths), true }},
	{"deaths10", false, false, 1, func(g *Game, _ *farmGame) (float64, bool) { return float64(g.Deaths10), true }},
	{"kp", true, false, 1, func(g *Game, _ *farmGame) (float64, bool) {
		if g.TeamKills == 0 {
			return 0, false
		}
		return float64(g.Kills+g.Assists) / float64(g.TeamKills), true
	}},
	{"lane_nw", true, false, 1, func(g *Game, _ *farmGame) (float64, bool) {
		if g.Opp == nil || g.NW10 == nil {
			return 0, false
		}
		return float64(*g.NW10 - g.Opp.NW10), true
	}},
	{"mid_idle", false, false, 1.1, func(_ *Game, fg *farmGame) (float64, bool) {
		if fg == nil || !fg.phases[1].ok {
			return 0, false
		}
		return fg.phases[1].idlePct(), true
	}},
	{"farm_gpm", true, true, 1, func(_ *Game, fg *farmGame) (float64, bool) {
		if fg == nil || fg.farmGPM <= 0 {
			return 0, false
		}
		return fg.farmGPM, true
	}},
}

func metricByKey(key string) (planMetric, bool) {
	for _, m := range planMetrics {
		if m.key == key {
			return m, true
		}
	}
	return planMetric{}, false
}

// planGames loads the account's parsed games (oldest first) with farm numbers.
func planGames(st *store.Store, accountID uint32, threshold float64) ([]Game, map[int64]farmGame, error) {
	games, err := loadGames(st, accountID)
	if err != nil {
		return nil, nil, err
	}
	series, err := st.FarmSeries(accountID)
	if err != nil {
		return nil, nil, err
	}
	return games, farmGames(games, series, threshold), nil
}

// NewPlan picks the goals from the recent games; nil when there are too few.
func NewPlan(games []Game, fgs map[int64]farmGame, now time.Time) *Plan {
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
		if m.core && support {
			continue
		}
		var vals []float64
		for i := range games {
			fg, ok := fgs[games[i].MatchID]
			var fp *farmGame
			if ok {
				fp = &fg
			}
			if v, ok := m.value(&games[i], fp); ok {
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
		cands = append(cands, cand{
			goal:  PlanGoal{Key: m.key, HigherBetter: m.higherBetter, Base: base, Target: base + (best-base)/2},
			score: gap / scale * m.weight,
		})
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
func PlanProgress(p *Plan, games []Game, fgs map[int64]farmGame, now time.Time) PlanView {
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
			if g.OpenDota || g.EndTime < p.Start || g.EndTime >= p.End {
				continue
			}
			if !ok {
				continue
			}
			fg, has := fgs[g.MatchID]
			var fp *farmGame
			if has {
				fp = &fg
			}
			val, okv := m.value(g, fp)
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
	games, fgs, err := planGames(st, accountID, threshold)
	if err != nil {
		return PlanView{}, err
	}
	if !found || len(p.Goals) == 0 {
		v := PlanProgress(nil, games, fgs, now)
		v.Enough = NewPlan(games, fgs, now) != nil
		return v, nil
	}
	return PlanProgress(&p, games, fgs, now), nil
}

// MakePlan builds and stores a new plan from the recent games.
func MakePlan(st *store.Store, accountID uint32, threshold float64, now time.Time) (PlanView, error) {
	if accountID == 0 {
		return PlanView{Goals: []GoalProgress{}, Unknown: true}, nil
	}
	games, fgs, err := planGames(st, accountID, threshold)
	if err != nil {
		return PlanView{}, err
	}
	p := NewPlan(games, fgs, now)
	if p == nil {
		return PlanView{Goals: []GoalProgress{}, Enough: false}, nil
	}
	if err := st.SetSetting(PlanKey, p); err != nil {
		return PlanView{}, err
	}
	return PlanProgress(p, games, fgs, now), nil
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
