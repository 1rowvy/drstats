// Package stats builds views and aggregates over stored matches.
package stats

import (
	"math"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

// Lane-opponent detection window (game seconds): the laning stage.
const (
	laneFrom = 60.0
	laneTo   = 600.0
)

// PlayerView is a scoreboard line with derived rates.
type PlayerView struct {
	parser.Player
	GPM int `json:"gpm"`
	XPM int `json:"xpm"`
	// OD are the OpenDota extras (estimated position, rank, party,
	// benchmarks); nil when not fetched.
	OD *opendota.Player `json:"od"`
}

// Line is a time series aligned with Farm.T; nil entries are "no value".
type Line []*float64

// Metric holds the compared lines of one chart.
type Metric struct {
	Focus    Line `json:"focus"`
	TeamAvg  Line `json:"team_avg"` // allies of the focused player, excluding them
	Opponent Line `json:"opponent"` // lane opponent; empty when unknown
}

// Farm are the series for the Farm tab.
type Farm struct {
	T        []float64 `json:"t"`
	NetWorth Metric    `json:"net_worth"`
	GPM      Metric    `json:"gpm"`
	XPM      Metric    `json:"xpm"`
	LastHits Metric    `json:"last_hits"`
	// Advantage is Radiant minus Dire net worth.
	Advantage Line `json:"advantage"`
	// Ticks are the replay ticks of T (for demo_gototick).
	Ticks []uint32 `json:"ticks"`
}

// MatchView is everything the match page needs.
type MatchView struct {
	Meta    store.MatchMeta `json:"meta"`
	Players []PlayerView    `json:"players"`
	// Focus is the analysed player id: "me" by account id, or the one the user
	// picked; -1 when neither is available.
	Focus    int            `json:"focus"`
	IsMe     bool           `json:"is_me"`
	Opponent int            `json:"opponent"` // lane opponent player id, -1 if unknown
	Farm     Farm           `json:"farm"`
	Deaths   []parser.Death `json:"deaths"`
	// DeathCtx is aligned with Deaths: where it happened and whether the
	// victim's team had an observer ward covering the spot.
	DeathCtx []DeathCtx `json:"death_ctx"`
	// ODStatus is the OpenDota fetch state: "" (not yet), ok, unparsed, notfound, error.
	ODStatus string     `json:"od_status"`
	World    [2]float64 `json:"world"` // map coordinate bounds [min, max]

	// Analyses of the focused player (zero values when Focus < 0).
	Laning    Laning       `json:"laning"`
	Gold      Gold         `json:"gold"`
	Items     []ItemTiming `json:"items"`
	Fights    Fights       `json:"fights"`
	Abilities Abilities    `json:"abilities"`
	Vision    Vision       `json:"vision"`

	Objectives []ObjectiveView `json:"objectives"`
	Battles    []Battle        `json:"battles"` // fights the focused player took part in
	// Grade of the focused player's game; nil without enough own matches.
	Grade *Grade `json:"grade"`
	// Phases of this game for the focused player (lane / mid / late) and the
	// thrown / comeback swing, nil when the game had none.
	Phases []PhaseStats `json:"phases"`
	Swing  *SwingGame   `json:"swing"`
	// Reasons: why the game was lost / won against the player's own games;
	// nil with too few of them.
	Reasons *MatchReasons `json:"reasons"`
}

// BuildMatchView loads a match and derives the view for the given focus:
// focusPID >= 0 picks a player explicitly, otherwise accountID selects "me".
func BuildMatchView(st *store.Store, matchID int64, accountID uint32, focusPID int, opt Options) (*MatchView, error) {
	meta, m, err := st.Match(matchID)
	if err != nil {
		return nil, err
	}
	players, series, deaths := m.Players, m.Series, m.Deaths
	v := &MatchView{Meta: *meta, Focus: -1, Opponent: -1, Deaths: deaths, World: [2]float64{parser.WorldMin, parser.WorldMax}}
	if v.Deaths == nil {
		v.Deaths = []parser.Death{}
	}
	od, err := st.OpenDotaPlayers(matchID)
	if err != nil {
		return nil, err
	}
	if v.ODStatus, _, err = st.OpenDotaStatus(matchID); err != nil {
		return nil, err
	}
	minutes := meta.Duration / 60
	for _, p := range players {
		pv := PlayerView{Player: p}
		if x, ok := od[p.HeroID]; ok {
			pv.OD = &x
		}
		if minutes > 0 {
			pv.GPM = int(math.Round(float64(p.TotalGold) / minutes))
			pv.XPM = int(math.Round(float64(p.TotalXP) / minutes))
		}
		v.Players = append(v.Players, pv)
		if focusPID < 0 && accountID != 0 && p.AccountID == accountID {
			v.Focus, v.IsMe = p.PlayerID, true
		}
	}
	if focusPID >= 0 {
		for _, p := range players {
			if p.PlayerID == focusPID {
				v.Focus = focusPID
				v.IsMe = accountID != 0 && p.AccountID == accountID
			}
		}
	}

	clock := parser.NewClock(m.StartTick, m.Pauses)
	byPID := splitSeries(series)
	v.DeathCtx = deathContext(players, deaths, m.Wards, byPID)
	for i := range v.DeathCtx {
		v.DeathCtx[i].Tick = clock.Tick(math.Max(deaths[i].T-8, -90))
	}

	if v.Focus >= 0 {
		v.Opponent = LaneOpponent(players, byPID, v.Focus)
	}
	v.Farm = buildFarm(players, byPID, v.Focus, v.Opponent)
	v.Farm.Advantage = advantage(players, byPID, v.Farm.T)
	v.Farm.Ticks = make([]uint32, len(v.Farm.T))
	for i, t := range v.Farm.T {
		v.Farm.Ticks[i] = clock.Tick(t)
	}
	v.Objectives = []ObjectiveView{}
	for _, o := range m.Objectives {
		v.Objectives = append(v.Objectives, ObjectiveView{Objective: o, Tick: clock.Tick(math.Max(o.T-10, -90))})
	}
	v.Items = []ItemTiming{}
	v.Battles = []Battle{}
	if v.Focus < 0 {
		return v, nil
	}
	var me parser.Player
	for _, p := range players {
		if p.PlayerID == v.Focus {
			me = p
		}
	}
	ss := byPID[v.Focus]
	v.Laning = buildLaning(m, byPID, v.Focus, v.Opponent)
	v.Gold = buildGold(me, ss, v.Farm.T, opt)
	var avg map[string]ItemAvg
	if me.AccountID != 0 {
		rows, err := st.ItemTimings(me.AccountID, me.HeroID, matchID)
		if err != nil {
			return nil, err
		}
		avg = map[string]ItemAvg{}
		for _, r := range rows {
			avg[r.Item] = ItemAvg{Avg: r.Avg, N: r.N}
		}
	}
	v.Items = buildItems(m, v.Focus, avg)
	v.Fights = buildFights(m, me)
	v.Abilities = buildAbilities(m, v.Focus, opt)
	v.Vision = buildVision(m, me, ss, v.Laning.Lane)
	v.Battles = buildBattles(m, players, byPID, v.Focus, clock)
	flow := matchFlow(players, ss, deaths, v.Farm, me)
	v.Phases = GamePhases(flow, meta.Duration)
	g := Game{GameRow: store.GameRow{MatchID: matchID, EndTime: meta.EndTime, Duration: meta.Duration,
		HeroName: me.HeroName, TotalGold: me.TotalGold}, Win: me.Team == meta.Winner}
	v.Swing = gameSwing(flow, &g)
	if me.AccountID != 0 {
		games, err := loadGames(st, me.AccountID)
		if err != nil {
			return nil, err
		}
		if g, ok := Grades(games)[matchID]; ok {
			v.Grade = &g
		}
		ex, err := loadExtras(st, me.AccountID, games, opt.IdleThreshold)
		if err != nil {
			return nil, err
		}
		v.Reasons = buildReasons(games, ex, matchID)
	}
	return v, nil
}

func splitSeries(series []parser.Sample) map[int][]parser.Sample {
	out := map[int][]parser.Sample{}
	for _, s := range series {
		out[s.PlayerID] = append(out[s.PlayerID], s)
	}
	return out
}

func teamOf(players []parser.Player, pid int) int {
	for _, p := range players {
		if p.PlayerID == pid {
			return p.Team
		}
	}
	return 0
}

// LaneOpponent returns the enemy who spent the laning stage closest to the
// focused player (mean distance over samples where both were alive), or -1.
func LaneOpponent(players []parser.Player, byPID map[int][]parser.Sample, focus int) int {
	team := teamOf(players, focus)
	cands := map[int][]parser.Sample{}
	for _, p := range players {
		if p.Team != team && p.Team != 0 {
			cands[p.PlayerID] = byPID[p.PlayerID]
		}
	}
	return closestRival(byPID[focus], cands)
}

// closestRival picks the candidate that stayed closest to mine on average
// during the laning stage (both alive, at least 10 shared samples); -1 if none.
func closestRival(mineSamples []parser.Sample, cands map[int][]parser.Sample) int {
	mine := map[float64]parser.Sample{}
	for _, s := range mineSamples {
		if s.T >= laneFrom && s.T <= laneTo && s.Alive {
			mine[s.T] = s
		}
	}
	best, bestDist := -1, math.Inf(1)
	for pid, ss := range cands {
		var sum float64
		var n int
		for _, s := range ss {
			m, ok := mine[s.T]
			if !ok || !s.Alive {
				continue
			}
			sum += math.Hypot(s.X-m.X, s.Y-m.Y)
			n++
		}
		if n < 10 {
			continue
		}
		if d := sum / float64(n); d < bestDist || (d == bestDist && pid < best) {
			best, bestDist = pid, d
		}
	}
	return best
}

func buildFarm(players []parser.Player, byPID map[int][]parser.Sample, focus, opp int) Farm {
	// Time axis: the longest player series (all are sampled on the same grid).
	var axis []float64
	for _, ss := range byPID {
		if len(ss) > len(axis) {
			axis = axis[:0]
			for _, s := range ss {
				axis = append(axis, s.T)
			}
		}
	}
	f := Farm{T: axis}
	if axis == nil {
		f.T = []float64{}
	}
	index := map[float64]int{}
	for i, t := range axis {
		index[t] = i
	}
	// value(pid, metric) -> line aligned with axis.
	type getter func(s parser.Sample) *float64
	line := func(pid int, get getter) Line {
		out := make(Line, len(axis))
		for _, s := range byPID[pid] {
			if i, ok := index[s.T]; ok {
				out[i] = get(s)
			}
		}
		return out
	}
	val := func(x float64) *float64 { return &x }
	perMin := func(x float64, t float64) *float64 {
		if t < 60 {
			return nil // rates in the first minute are noise
		}
		return val(math.Round(x / (t / 60)))
	}
	getters := map[*Metric]getter{
		&f.NetWorth: func(s parser.Sample) *float64 { return val(float64(s.NetWorth)) },
		&f.GPM:      func(s parser.Sample) *float64 { return perMin(float64(s.TotalGold), s.T) },
		&f.XPM:      func(s parser.Sample) *float64 { return perMin(float64(s.TotalXP), s.T) },
		&f.LastHits: func(s parser.Sample) *float64 { return val(float64(s.LastHits)) },
	}
	team := teamOf(players, focus)
	var allies []int
	for _, p := range players {
		if p.Team == team && p.PlayerID != focus {
			allies = append(allies, p.PlayerID)
		}
	}
	for m, get := range getters {
		m.Focus, m.TeamAvg, m.Opponent = Line{}, Line{}, Line{}
		if focus < 0 {
			continue
		}
		m.Focus = line(focus, get)
		if opp >= 0 {
			m.Opponent = line(opp, get)
		}
		m.TeamAvg = average(axis, allies, func(pid int) Line { return line(pid, get) })
	}
	return f
}

func average(axis []float64, pids []int, line func(int) Line) Line {
	out := make(Line, len(axis))
	if len(pids) == 0 {
		return out
	}
	sums := make([]float64, len(axis))
	counts := make([]int, len(axis))
	for _, pid := range pids {
		for i, v := range line(pid) {
			if v != nil {
				sums[i] += *v
				counts[i]++
			}
		}
	}
	for i := range out {
		if counts[i] > 0 {
			v := math.Round(sums[i] / float64(counts[i]))
			out[i] = &v
		}
	}
	return out
}
