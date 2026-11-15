// Package stats builds views and aggregates over stored matches.
package stats

import (
	"math"

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
	World    [2]float64     `json:"world"` // map coordinate bounds [min, max]
}

// BuildMatchView loads a match and derives the view for the given focus:
// focusPID >= 0 picks a player explicitly, otherwise accountID selects "me".
func BuildMatchView(st *store.Store, matchID int64, accountID uint32, focusPID int) (*MatchView, error) {
	meta, players, series, deaths, err := st.Match(matchID)
	if err != nil {
		return nil, err
	}
	v := &MatchView{Meta: *meta, Focus: -1, Opponent: -1, Deaths: deaths, World: [2]float64{parser.WorldMin, parser.WorldMax}}
	if v.Deaths == nil {
		v.Deaths = []parser.Death{}
	}
	minutes := meta.Duration / 60
	for _, p := range players {
		pv := PlayerView{Player: p}
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

	byPID := splitSeries(series)
	if v.Focus >= 0 {
		v.Opponent = LaneOpponent(players, byPID, v.Focus)
	}
	v.Farm = buildFarm(players, byPID, v.Focus, v.Opponent)
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
	mine := map[float64]parser.Sample{}
	for _, s := range byPID[focus] {
		if s.T >= laneFrom && s.T <= laneTo && s.Alive {
			mine[s.T] = s
		}
	}
	best, bestDist := -1, math.Inf(1)
	for _, p := range players {
		if p.Team == team || p.Team == 0 {
			continue
		}
		var sum float64
		var n int
		for _, s := range byPID[p.PlayerID] {
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
		if d := sum / float64(n); d < bestDist {
			best, bestDist = p.PlayerID, d
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
