package stats

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

// Filter narrows the matches an aggregate is computed over. Zero values mean "any".
type Filter struct {
	HeroID int    `json:"hero_id"`
	Days   int    `json:"days"`   // only matches of the last N days
	Last   int    `json:"last"`   // only the last N matches (after the other filters)
	Result string `json:"result"` // "win" / "loss"
	Mode   int    `json:"mode"`   // game_mode
	Lane   string `json:"lane"`   // detected lane
	Source string `json:"source"` // SourceReplays / SourceAll
}

// Game is a GameRow with derived values.
type Game struct {
	store.GameRow
	Win  bool   `json:"win"`
	Lane string `json:"lane"`
	// OpenDota marks a match known only from the OpenDota history (no replay).
	OpenDota bool `json:"opendota"`
	// Lane opponent at 10:00 (nil when unknown) and the lane verdict.
	Opp        *LaneOpp `json:"opp"`
	LaneScore  float64  `json:"lane_score"`
	LaneResult string   `json:"lane_result"` // LaneWon / LaneEven / LaneLost, "" when unknown
}

// Metric definitions for averages, trends and habits.
type metricDef struct {
	Key          string
	HigherBetter bool
	Value        func(g *Game) (float64, bool)
}

func perMin(v int, g *Game) (float64, bool) {
	if g.Duration < 60 {
		return 0, false
	}
	return float64(v) / (g.Duration / 60), true
}

// replayOnly wraps a metric that only parsed replays have.
func replayOnly(v func(g *Game) float64) func(g *Game) (float64, bool) {
	return func(g *Game) (float64, bool) {
		if g.OpenDota {
			return 0, false
		}
		return v(g), true
	}
}

func intp(p *int) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return float64(*p), true
}

var metrics = []metricDef{
	{"kda", true, func(g *Game) (float64, bool) {
		return float64(g.Kills+g.Assists) / math.Max(1, float64(g.Deaths)), true
	}},
	{"kills", true, func(g *Game) (float64, bool) { return float64(g.Kills), true }},
	{"deaths", false, func(g *Game) (float64, bool) { return float64(g.Deaths), true }},
	{"assists", true, func(g *Game) (float64, bool) { return float64(g.Assists), true }},
	{"gpm", true, func(g *Game) (float64, bool) { return perMin(g.TotalGold, g) }},
	{"xpm", true, func(g *Game) (float64, bool) { return perMin(g.TotalXP, g) }},
	{"lh10", true, func(g *Game) (float64, bool) { return intp(g.LH10) }},
	{"dn10", true, func(g *Game) (float64, bool) { return intp(g.DN10) }},
	{"nw10", true, func(g *Game) (float64, bool) { return intp(g.NW10) }},
	{"deaths10", false, replayOnly(func(g *Game) float64 { return float64(g.Deaths10) })},
	{"kp", true, func(g *Game) (float64, bool) {
		if g.TeamKills == 0 {
			return 0, false
		}
		return math.Min(1, float64(g.Kills+g.Assists)/float64(g.TeamKills)), true
	}},
	{"hero_dmg_pm", true, func(g *Game) (float64, bool) {
		if g.OpenDota && g.HeroDamage == 0 { // OpenDota has no damage for old matches
			return 0, false
		}
		return perMin(g.HeroDamage, g)
	}},
	{"tower_dmg", true, func(g *Game) (float64, bool) { return float64(g.TowerDamage), true }},
	{"gold_lost", false, replayOnly(func(g *Game) float64 { return float64(g.GoldLost) })},
	{"wards", true, replayOnly(func(g *Game) float64 { return float64(g.ObsPlaced + g.SenPlaced) })},
	{"stacks", true, replayOnly(func(g *Game) float64 { return float64(g.CampsStacked) })},
}

// MetricAvg is an average of one metric.
type MetricAvg struct {
	Key string  `json:"key"`
	Avg float64 `json:"avg"`
	N   int     `json:"n"`
}

// Bucket is a winrate group (hero, weekday, hour, lane).
type Bucket struct {
	Key     string  `json:"key"`
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	Winrate float64 `json:"winrate"`
	KDA     float64 `json:"kda"`
	GPM     float64 `json:"gpm"`
}

// TrendGame is one match on the trend chart.
type TrendGame struct {
	MatchID  int64               `json:"match_id"`
	EndTime  int64               `json:"end_time"`
	Win      bool                `json:"win"`
	HeroName string              `json:"hero_name"`
	OpenDota bool                `json:"opendota"` // no replay
	Values   map[string]*float64 `json:"values"`
}

// Habit compares the last HabitRecent matches with the earlier ones.
type Habit struct {
	Key          string  `json:"key"`
	HigherBetter bool    `json:"higher_better"`
	Recent       float64 `json:"recent"`
	Before       float64 `json:"before"`
	Change       float64 `json:"change"` // relative, signed so that negative = worse
	Worse        bool    `json:"worse"`
}

// Habit thresholds.
const (
	HabitRecent    = 10   // matches in the "recent" window
	HabitMinBefore = 5    // matches needed before the window
	HabitWorse     = 0.10 // relative change that counts as a worse habit
	TrendLength    = 30   // matches on the trend chart
)

// HeroOption is a hero the account has played (for filter pickers).
type HeroOption struct {
	HeroID   int    `json:"hero_id"`
	HeroName string `json:"hero_name"`
	Games    int    `json:"games"`
}

// HeroExtras are the hero-page-only aggregates.
type HeroExtras struct {
	Items     []store.ItemTiming   `json:"items"`     // average first purchase time
	Abilities []store.AbilityTotal `json:"abilities"` // casts over all matches on the hero
}

// Dashboard is the aggregate view over the filtered matches.
type Dashboard struct {
	Filter    Filter             `json:"filter"`
	Games     int                `json:"games"`
	Wins      int                `json:"wins"`
	Winrate   float64            `json:"winrate"`
	Averages  []MetricAvg        `json:"averages"`
	ByHero    []Bucket           `json:"by_hero"`    // key: hero npc name
	ByWeekday []Bucket           `json:"by_weekday"` // key: 1 (Mon) .. 7 (Sun), local time of the game start
	ByHour    []Bucket           `json:"by_hour"`    // key: 0..23
	ByLane    []Bucket           `json:"by_lane"`
	Trend     []TrendGame        `json:"trend"` // last TrendLength matches, oldest first
	Habits    []Habit            `json:"habits"`
	Recent    []Game             `json:"recent"` // newest first
	Deaths    []store.DeathPoint `json:"deaths"` // positioned deaths in the filtered matches
	Heroes    []HeroOption       `json:"heroes"` // all heroes played (unfiltered)
	Modes     []int              `json:"modes"`  // all game modes played (unfiltered)
	Hero      *HeroExtras        `json:"hero"`   // set when Filter.HeroID != 0
	World     [2]float64         `json:"world"`
	Total     int                `json:"total"`   // matches before filtering
	Unknown   bool               `json:"unknown"` // no Steam ID configured
	Extras    Extras             `json:"extras"`
	Lanes     LaneStats          `json:"lanes"` // against the lane opponent
	// Replays is how many of the filtered matches have a parsed replay
	// (replay-only blocks are computed over those).
	Replays int `json:"replays"`
	// History is the OpenDota history sync state (nil without a Steam ID).
	History *store.HistorySync `json:"history"`
}

func worldBounds() [2]float64 { return [2]float64{parser.WorldMin, parser.WorldMax} }

// BuildDashboard aggregates the account's matches under a filter. heroByID
// names the heroes of OpenDota history matches (constants hero_by_id).
func BuildDashboard(st *store.Store, accountID uint32, f Filter, heroByID map[int]string) (*Dashboard, error) {
	d := &Dashboard{Filter: f, Averages: []MetricAvg{}, ByHero: []Bucket{}, ByWeekday: []Bucket{}, ByHour: []Bucket{},
		ByLane: []Bucket{}, Trend: []TrendGame{}, Habits: []Habit{}, Recent: []Game{}, Deaths: []store.DeathPoint{},
		Heroes: []HeroOption{}, Modes: []int{}, World: worldBounds(), Unknown: accountID == 0,
		Lanes: LaneStats{Matchups: []LaneMatchup{}}}
	all, err := loadGames(st, accountID)
	if err != nil {
		return nil, err
	}
	if accountID != 0 {
		hist, err := st.History(accountID)
		if err != nil {
			return nil, err
		}
		state, err := st.HistoryState(accountID)
		if err != nil {
			return nil, err
		}
		d.History = &state
		all = mergeHistory(all, hist, HeroNamer(heroByID), f.Source == SourceAll)
	}
	d.Total = len(all)
	heroes := map[int]*HeroOption{}
	modes := map[int]bool{}
	for _, r := range all {
		h := heroes[r.HeroID]
		if h == nil {
			h = &HeroOption{HeroID: r.HeroID, HeroName: r.HeroName}
			heroes[r.HeroID] = h
		}
		h.Games++
		modes[r.GameMode] = true
	}
	for _, h := range heroes {
		d.Heroes = append(d.Heroes, *h)
	}
	sort.Slice(d.Heroes, func(i, j int) bool { return d.Heroes[i].Games > d.Heroes[j].Games })
	for m := range modes {
		d.Modes = append(d.Modes, m)
	}
	sort.Ints(d.Modes)

	games := applyFilter(all, f, time.Now())
	d.Games = len(games)
	inFilter := map[int64]bool{}
	for i := range games {
		inFilter[games[i].MatchID] = true
		if !games[i].OpenDota {
			d.Replays++
		}
		if games[i].Win {
			d.Wins++
		}
	}
	if d.Games > 0 {
		d.Winrate = float64(d.Wins) / float64(d.Games)
	}
	d.Averages = averages(games)
	d.ByHero = buckets(games, func(g *Game) string { return g.HeroName })
	sort.SliceStable(d.ByHero, func(i, j int) bool { return d.ByHero[i].Games > d.ByHero[j].Games })
	d.ByWeekday = fixedBuckets(games, 1, 7, func(g *Game) int {
		wd := int(gameStart(g).Weekday())
		if wd == 0 {
			wd = 7
		}
		return wd
	})
	d.ByHour = fixedBuckets(games, 0, 23, func(g *Game) int { return gameStart(g).Hour() })
	d.ByLane = buckets(games, func(g *Game) string { return g.Lane })
	sort.SliceStable(d.ByLane, func(i, j int) bool { return d.ByLane[i].Games > d.ByLane[j].Games })
	d.Trend = trend(games)
	d.Habits = habits(games)
	for i := len(games) - 1; i >= 0 && len(d.Recent) < 10; i-- {
		d.Recent = append(d.Recent, games[i])
	}

	d.Extras = buildExtras(games, time.Now())
	d.Lanes = laneStats(games)
	if accountID != 0 {
		fast, err := st.FastestItems(accountID)
		if err != nil {
			return nil, err
		}
		byID := map[int64]*Game{}
		for i := range games {
			byID[games[i].MatchID] = &games[i]
		}
		for _, it := range fast {
			if g, ok := byID[it.MatchID]; ok {
				d.Extras.Fastest = append(d.Extras.Fastest, FastItemView{FastItem: it, HeroName: g.HeroName, EndTime: g.EndTime})
			}
		}
		deaths, err := st.DeathPoints(accountID, f.HeroID)
		if err != nil {
			return nil, err
		}
		for _, p := range deaths {
			if inFilter[p.MatchID] {
				p.Region = Region(p.X, p.Y)
				d.Deaths = append(d.Deaths, p)
			}
		}
		if f.HeroID != 0 {
			items, err := st.ItemTimings(accountID, f.HeroID, 0)
			if err != nil {
				return nil, err
			}
			abilities, err := st.AbilityCasts(accountID, f.HeroID)
			if err != nil {
				return nil, err
			}
			if items == nil {
				items = []store.ItemTiming{}
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Avg < items[j].Avg })
			d.Hero = &HeroExtras{Items: items, Abilities: abilities}
		}
	}
	return d, nil
}

// loadGames returns the account's matches with derived values, oldest first.
func loadGames(st *store.Store, accountID uint32) ([]Game, error) {
	rows, err := st.Games(accountID)
	if err != nil {
		return nil, err
	}
	all := make([]Game, len(rows))
	for i, r := range rows {
		all[i] = Game{GameRow: r, Win: r.Team == r.Winner, Lane: DetectLane(r.LaneEarly)}
	}
	rivals, err := st.LaneRivals(accountID)
	if err != nil {
		return nil, err
	}
	attachLaneOpps(all, rivals)
	return all, nil
}

// MatchGrades grades every match of the account (see Grades).
func MatchGrades(st *store.Store, accountID uint32) (map[int64]Grade, error) {
	games, err := loadGames(st, accountID)
	if err != nil {
		return nil, err
	}
	return Grades(games), nil
}

func gameStart(g *Game) time.Time {
	return time.Unix(g.EndTime-int64(g.Duration), 0).Local()
}

func applyFilter(all []Game, f Filter, now time.Time) []Game {
	var out []Game
	cutoff := int64(0)
	if f.Days > 0 {
		cutoff = now.Add(-time.Duration(f.Days) * 24 * time.Hour).Unix()
	}
	for _, g := range all {
		switch {
		case f.HeroID != 0 && g.HeroID != f.HeroID,
			cutoff != 0 && g.EndTime < cutoff,
			f.Result == "win" && !g.Win,
			f.Result == "loss" && g.Win,
			f.Mode != 0 && g.GameMode != f.Mode,
			f.Lane != "" && g.Lane != f.Lane:
			continue
		}
		out = append(out, g)
	}
	if f.Last > 0 && len(out) > f.Last {
		out = out[len(out)-f.Last:]
	}
	return out
}

func mean(games []Game, m metricDef) (float64, int) {
	var sum float64
	n := 0
	for i := range games {
		if v, ok := m.Value(&games[i]); ok {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
}

func averages(games []Game) []MetricAvg {
	out := []MetricAvg{}
	for _, m := range metrics {
		avg, n := mean(games, m)
		out = append(out, MetricAvg{Key: m.Key, Avg: avg, N: n})
	}
	return out
}

func addBucket(b *Bucket, g *Game) {
	b.Games++
	if g.Win {
		b.Wins++
	}
	kda, _ := metrics[0].Value(g)
	gpm, _ := perMin(g.TotalGold, g)
	b.KDA += kda
	b.GPM += gpm
}

func finishBucket(b *Bucket) {
	if b.Games > 0 {
		b.Winrate = float64(b.Wins) / float64(b.Games)
		b.KDA /= float64(b.Games)
		b.GPM /= float64(b.Games)
	}
}

func buckets(games []Game, key func(*Game) string) []Bucket {
	idx := map[string]int{}
	var out []Bucket
	for i := range games {
		k := key(&games[i])
		j, ok := idx[k]
		if !ok {
			j = len(out)
			idx[k] = j
			out = append(out, Bucket{Key: k})
		}
		addBucket(&out[j], &games[i])
	}
	for i := range out {
		finishBucket(&out[i])
	}
	if out == nil {
		out = []Bucket{}
	}
	return out
}

func fixedBuckets(games []Game, from, to int, key func(*Game) int) []Bucket {
	out := make([]Bucket, to-from+1)
	for i := range out {
		out[i].Key = strconv.Itoa(from + i)
	}
	for i := range games {
		addBucket(&out[key(&games[i])-from], &games[i])
	}
	for i := range out {
		finishBucket(&out[i])
	}
	return out
}

func trend(games []Game) []TrendGame {
	start := max(0, len(games)-TrendLength)
	out := []TrendGame{}
	for i := start; i < len(games); i++ {
		g := &games[i]
		tg := TrendGame{MatchID: g.MatchID, EndTime: g.EndTime, Win: g.Win, HeroName: g.HeroName, OpenDota: g.OpenDota, Values: map[string]*float64{}}
		for _, m := range metrics {
			if v, ok := m.Value(g); ok {
				tg.Values[m.Key] = &v
			} else {
				tg.Values[m.Key] = nil
			}
		}
		out = append(out, tg)
	}
	return out
}

// habits compares the last HabitRecent matches with all earlier filtered ones.
func habits(games []Game) []Habit {
	out := []Habit{}
	if len(games) < HabitRecent+HabitMinBefore {
		return out
	}
	before, recent := games[:len(games)-HabitRecent], games[len(games)-HabitRecent:]
	for _, m := range metrics {
		b, nb := mean(before, m)
		r, nr := mean(recent, m)
		if nb < HabitMinBefore || nr == 0 || b == 0 {
			continue
		}
		change := (r - b) / math.Abs(b)
		if !m.HigherBetter {
			change = -change
		}
		out = append(out, Habit{Key: m.Key, HigherBetter: m.HigherBetter, Recent: r, Before: b, Change: change,
			Worse: change <= -HabitWorse})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Change < out[j].Change })
	return out
}
