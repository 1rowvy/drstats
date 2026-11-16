package stats

import (
	"math"
	"sort"
	"strconv"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

// Game phases (game seconds). The late game runs to the end of the match.
const (
	PhaseMidFrom  = 600
	PhaseLateFrom = 1500
	// PhaseMinLen is how long a phase must have lasted to be counted.
	PhaseMinLen = 180
	// ThrowLead is the team net worth lead that counts as "had the game":
	// lost after it = thrown, won after being that far behind = comeback.
	ThrowLead = 5000
	// FlowListLen is how many thrown / comeback games are listed.
	FlowListLen = 5
)

// PhaseStats are the averages of one game phase over the games that reached it.
type PhaseStats struct {
	Key       string  `json:"key"` // lane / mid / late
	Games     int     `json:"games"`
	GPM       float64 `json:"gpm"`
	XPM       float64 `json:"xpm"`
	LHPM      float64 `json:"lhpm"`       // last hits per minute
	Deaths10  float64 `json:"deaths10"`   // deaths per 10 minutes
	KP        float64 `json:"kp"`         // share of the team's kills with your kill or assist
	LeadDelta float64 `json:"lead_delta"` // average change of the team lead during the phase
	Gained    int     `json:"gained"`     // games where the team lead grew in the phase
	// Winrate of the games where the team gained / lost lead in this phase.
	WinGained float64 `json:"win_gained"`
	WinLost   float64 `json:"win_lost"`
}

// SwingGame is a thrown or comeback game.
type SwingGame struct {
	MatchID  int64   `json:"match_id"`
	HeroName string  `json:"hero_name"`
	EndTime  int64   `json:"end_time"`
	Win      bool    `json:"win"`
	Peak     int     `json:"peak"`   // biggest lead (thrown) or deficit (comeback), absolute
	PeakT    float64 `json:"peak_t"` // when it was
	FlipT    float64 `json:"flip_t"` // when the lead changed sides afterwards (-1 if it never did)
	// Your play between PeakT and FlipT (or the end).
	Deaths  int     `json:"deaths"`
	KP      float64 `json:"kp"` // -1 when the team got no kills
	GPM     float64 `json:"gpm"`
	GameGPM float64 `json:"game_gpm"` // your GPM over the whole game
}

// FlowStats summarise how games develop after the lane.
type FlowStats struct {
	Games  int          `json:"games"` // games with a parsed replay
	Phases []PhaseStats `json:"phases"`
	// Games with a lead of ThrowLead+ / a deficit of ThrowLead+ and how they ended.
	Ahead     Bucket      `json:"ahead"`
	Behind    Bucket      `json:"behind"`
	Thrown    []SwingGame `json:"thrown"`    // newest first, up to FlowListLen
	Comebacks []SwingGame `json:"comebacks"` // newest first, up to FlowListLen
	// Thrown-game averages of your play during the collapse.
	ThrowDeaths float64 `json:"throw_deaths"`
	ThrowKP     float64 `json:"throw_kp"`
	// Winrate by game length (all games, history included). Key: lower bound in minutes.
	ByLength []Bucket `json:"by_length"`
}

var lengthEdges = []int{0, 25, 35, 45}

func lengthBuckets(games []Game) []Bucket {
	out := make([]Bucket, len(lengthEdges))
	for i, e := range lengthEdges {
		out[i].Key = itoa(e)
	}
	for i := range games {
		g := &games[i]
		min := int(g.Duration / 60)
		k := 0
		for j, e := range lengthEdges {
			if min >= e {
				k = j
			}
		}
		addBucket(&out[k], g)
	}
	for i := range out {
		finishBucket(&out[i])
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }

// sampleBefore returns the last flow sample at or before t.
func sampleBefore(ss []store.FlowSample, t float64) (store.FlowSample, bool) {
	i := sort.Search(len(ss), func(i int) bool { return ss[i].T > t })
	if i == 0 {
		return store.FlowSample{}, false
	}
	return ss[i-1], true
}

func leadAt(lp []store.LeadPoint, t float64) (int, bool) {
	i := sort.Search(len(lp), func(i int) bool { return lp[i].T > t })
	if i == 0 {
		return 0, false
	}
	return lp[i-1].Lead, true
}

// window counts your deaths, kill participation and gold between from and to.
func window(f *store.GameFlow, from, to float64) (deaths, credit, teamKills int, gpm float64, ok bool) {
	a, okA := sampleBefore(f.Samples, from)
	b, okB := sampleBefore(f.Samples, to)
	if !okA || !okB || b.T-a.T < 60 {
		return 0, 0, 0, 0, false
	}
	gpm = float64(b.TotalGold-a.TotalGold) / ((b.T - a.T) / 60)
	for _, d := range f.Deaths {
		if d.T < from || d.T >= to {
			continue
		}
		if d.Mine {
			deaths++
		}
		if !d.Ally {
			teamKills++
			if d.Credit {
				credit++
			}
		}
	}
	return deaths, credit, teamKills, gpm, true
}

// phaseGame are one game's values in one phase.
type phaseGame struct {
	ok                       bool
	gpm, xpm, lhpm, deaths10 float64
	kp                       float64 // -1 when the team got no kills
	leadDelta                int
}

var phaseKeys = []string{"lane", "mid", "late"}
var phaseBounds = [][2]float64{{0, PhaseMidFrom}, {PhaseMidFrom, PhaseLateFrom}, {PhaseLateFrom, math.Inf(1)}}

// gamePhases splits one game (ending at end, game seconds) into phases.
func gamePhases(f *store.GameFlow, end float64) []phaseGame {
	out := make([]phaseGame, len(phaseBounds))
	for k, b := range phaseBounds {
		from, to := b[0], math.Min(b[1], end)
		if to-from < PhaseMinLen {
			continue
		}
		deaths, credit, teamKills, gpm, ok := window(f, from, to)
		if !ok {
			continue
		}
		a, _ := sampleBefore(f.Samples, from)
		z, _ := sampleBefore(f.Samples, to)
		mins := (z.T - a.T) / 60
		pg := phaseGame{ok: true, gpm: gpm, kp: -1,
			xpm:      float64(z.TotalXP-a.TotalXP) / mins,
			lhpm:     float64(z.LastHits-a.LastHits) / mins,
			deaths10: float64(deaths) / mins * 10}
		if teamKills > 0 {
			pg.kp = float64(credit) / float64(teamKills)
		}
		l0, _ := leadAt(f.Lead, from)
		l1, _ := leadAt(f.Lead, to)
		pg.leadDelta = l1 - l0
		out[k] = pg
	}
	return out
}

// GamePhases are the phases of a single game (Games is 1 for a phase the
// game reached, KP is -1 when the team got no kills in it).
func GamePhases(f *store.GameFlow, end float64) []PhaseStats {
	out := []PhaseStats{}
	for k, pg := range gamePhases(f, end) {
		ps := PhaseStats{Key: phaseKeys[k]}
		if pg.ok {
			ps.Games = 1
			ps.GPM, ps.XPM, ps.LHPM, ps.Deaths10, ps.KP, ps.LeadDelta = pg.gpm, pg.xpm, pg.lhpm, pg.deaths10, pg.kp, float64(pg.leadDelta)
			if pg.leadDelta > 0 {
				ps.Gained = 1
			}
		}
		out = append(out, ps)
	}
	return out
}

// gameSwing returns the thrown (lost after leading by ThrowLead+) or
// comeback (won after trailing by ThrowLead+) part of a game, nil if neither.
func gameSwing(f *store.GameFlow, g *Game) *SwingGame {
	if len(f.Lead) == 0 {
		return nil
	}
	maxI, minI := 0, 0
	for j, lp := range f.Lead {
		if lp.Lead > f.Lead[maxI].Lead {
			maxI = j
		}
		if lp.Lead < f.Lead[minI].Lead {
			minI = j
		}
	}
	switch {
	case !g.Win && f.Lead[maxI].Lead >= ThrowLead:
		return swingFrom(f, g, maxI, 1)
	case g.Win && f.Lead[minI].Lead <= -ThrowLead:
		return swingFrom(f, g, minI, -1)
	}
	return nil
}

func swingFrom(f *store.GameFlow, g *Game, peakI, sign int) *SwingGame {
	peak := f.Lead[peakI]
	sg := &SwingGame{MatchID: g.MatchID, HeroName: g.HeroName, EndTime: g.EndTime, Win: g.Win,
		Peak: peak.Lead * sign, PeakT: peak.T, FlipT: -1, KP: -1}
	if gg, ok := perMin(g.TotalGold, g); ok {
		sg.GameGPM = gg
	}
	to := g.Duration
	for _, lp := range f.Lead[peakI:] {
		if lp.Lead*sign <= 0 {
			sg.FlipT, to = lp.T, lp.T
			break
		}
	}
	if d, c, tk, gpm, ok := window(f, peak.T, to); ok {
		sg.Deaths, sg.GPM = d, gpm
		if tk > 0 {
			sg.KP = float64(c) / float64(tk)
		}
	}
	return sg
}

func buildFlow(games []Game, flows map[int64]*store.GameFlow) FlowStats {
	fs := FlowStats{Phases: []PhaseStats{}, Thrown: []SwingGame{}, Comebacks: []SwingGame{},
		Ahead: Bucket{Key: "ahead"}, Behind: Bucket{Key: "behind"}, ByLength: lengthBuckets(games)}
	type acc struct {
		PhaseStats
		kpN               int
		winG, winL, lostN int
	}
	phases := make([]*acc, len(phaseKeys))
	for k, key := range phaseKeys {
		phases[k] = &acc{PhaseStats: PhaseStats{Key: key}}
	}
	var throwDeaths, throwKP float64
	var throwKPN int
	var thrown, comebacks []SwingGame
	for i := range games {
		g := &games[i]
		f := flows[g.MatchID]
		if g.OpenDota || f == nil || len(f.Samples) == 0 || len(f.Lead) == 0 {
			continue
		}
		fs.Games++
		for k, pg := range gamePhases(f, g.Duration) {
			if !pg.ok {
				continue
			}
			p := phases[k]
			p.Games++
			p.GPM += pg.gpm
			p.XPM += pg.xpm
			p.LHPM += pg.lhpm
			p.Deaths10 += pg.deaths10
			if pg.kp >= 0 {
				p.KP += pg.kp
				p.kpN++
			}
			p.LeadDelta += float64(pg.leadDelta)
			if pg.leadDelta > 0 {
				p.Gained++
				if g.Win {
					p.winG++
				}
			} else {
				p.lostN++
				if g.Win {
					p.winL++
				}
			}
		}
		maxLead, minLead := f.Lead[0].Lead, f.Lead[0].Lead
		for _, lp := range f.Lead {
			maxLead = max(maxLead, lp.Lead)
			minLead = min(minLead, lp.Lead)
		}
		if maxLead >= ThrowLead {
			addBucket(&fs.Ahead, g)
		}
		if minLead <= -ThrowLead {
			addBucket(&fs.Behind, g)
		}
		if sg := gameSwing(f, g); sg != nil {
			if sg.Win {
				comebacks = append(comebacks, *sg)
			} else {
				thrown = append(thrown, *sg)
				throwDeaths += float64(sg.Deaths)
				if sg.KP >= 0 {
					throwKP += sg.KP
					throwKPN++
				}
			}
		}
	}
	for _, p := range phases {
		if p.Games > 0 {
			n := float64(p.Games)
			p.GPM /= n
			p.XPM /= n
			p.LHPM /= n
			p.Deaths10 /= n
			p.LeadDelta /= n
		}
		if p.kpN > 0 {
			p.KP /= float64(p.kpN)
		}
		if p.Gained > 0 {
			p.WinGained = float64(p.winG) / float64(p.Gained)
		}
		if p.lostN > 0 {
			p.WinLost = float64(p.winL) / float64(p.lostN)
		}
		fs.Phases = append(fs.Phases, p.PhaseStats)
	}
	finishBucket(&fs.Ahead)
	finishBucket(&fs.Behind)
	if len(thrown) > 0 {
		fs.ThrowDeaths = throwDeaths / float64(len(thrown))
	}
	if throwKPN > 0 {
		fs.ThrowKP = throwKP / float64(throwKPN)
	}
	newest := func(gs []SwingGame) []SwingGame {
		sort.SliceStable(gs, func(i, j int) bool { return gs[i].EndTime > gs[j].EndTime })
		if len(gs) > FlowListLen {
			gs = gs[:FlowListLen]
		}
		if gs == nil {
			gs = []SwingGame{}
		}
		return gs
	}
	fs.Thrown = newest(thrown)
	fs.Comebacks = newest(comebacks)
	return fs
}

// matchFlow builds the GameFlow of one loaded match for the focused player:
// samples every minute, the team lead from the advantage line, all deaths.
func matchFlow(players []parser.Player, ss []parser.Sample, deaths []parser.Death, farm Farm, me parser.Player) *store.GameFlow {
	f := &store.GameFlow{}
	for _, s := range ss {
		if s.T >= 0 && math.Mod(s.T, 60) == 0 {
			f.Samples = append(f.Samples, store.FlowSample{T: s.T, TotalGold: s.TotalGold, TotalXP: s.TotalXP, LastHits: s.LastHits})
		}
	}
	sign := 1
	if me.Team == parser.TeamDire {
		sign = -1
	}
	for i, t := range farm.T {
		if t >= 0 && math.Mod(t, 60) == 0 && i < len(farm.Advantage) && farm.Advantage[i] != nil {
			f.Lead = append(f.Lead, store.LeadPoint{T: t, Lead: int(*farm.Advantage[i]) * sign})
		}
	}
	team := map[int]int{}
	for _, p := range players {
		team[p.PlayerID] = p.Team
	}
	for _, d := range deaths {
		fd := store.FlowDeath{T: d.T, Mine: d.VictimPID == me.PlayerID, Ally: team[d.VictimPID] == me.Team,
			Credit: d.KillerPID == me.PlayerID}
		if !fd.Ally {
			for _, a := range d.Assists {
				if a == me.PlayerID {
					fd.Credit = true
				}
			}
		}
		f.Deaths = append(f.Deaths, fd)
	}
	return f
}
