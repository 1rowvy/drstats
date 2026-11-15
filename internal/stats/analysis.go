package stats

import (
	"math"
	"sort"
	"strings"

	"github.com/1rowvy/drstats/internal/parser"
)

// Options are the user-tunable heuristic thresholds.
type Options struct {
	HitWindow     float64 // seconds after a cast in which damage/debuff counts as a hit
	IdleThreshold float64 // seconds without farm or fight that count as idle
}

// DefaultOptions mirror the settings defaults.
var DefaultOptions = Options{HitWindow: 1, IdleThreshold: 20}

// ---- laning ----

// LaneSnap compares the focused player with the lane opponent at a moment.
type LaneSnap struct {
	T        float64 `json:"t"`
	LH       int     `json:"lh"`
	DN       int     `json:"dn"`
	NetWorth int     `json:"nw"`
	XP       int     `json:"xp"`
	Level    int     `json:"level"`
	// Opponent values; nil when there is no opponent or no sample.
	Opp *LaneSnapValues `json:"opp"`
}

// LaneSnapValues are one player's values at a LaneSnap moment.
type LaneSnapValues struct {
	LH       int `json:"lh"`
	DN       int `json:"dn"`
	NetWorth int `json:"nw"`
	XP       int `json:"xp"`
	Level    int `json:"level"`
}

// Laning is the laning-stage summary (first 10 minutes).
type Laning struct {
	Lane     string         `json:"lane"`      // top / mid / bot / jungle / roam / ""
	OppLane  string         `json:"opp_lane"`  // lane of the opponent
	Snaps    []LaneSnap     `json:"snaps"`     // @5:00 and @10:00
	Deaths   []parser.Death `json:"deaths"`    // focus deaths before 10:00
	Kills    int            `json:"kills"`     // kills by focus before 10:00
	Assists  int            `json:"assists"`   // assists before 10:00
	LaneEnds float64        `json:"lane_ends"` // end of the laning window, seconds
}

func sampleAt(ss []parser.Sample, t float64) (parser.Sample, bool) {
	i := sort.Search(len(ss), func(i int) bool { return ss[i].T >= t })
	if i < len(ss) && ss[i].T-t < parser.SeriesStep {
		return ss[i], true
	}
	if len(ss) > 0 && i == len(ss) && t-ss[len(ss)-1].T < parser.SeriesStep {
		return ss[len(ss)-1], true
	}
	return parser.Sample{}, false
}

func buildLaning(m *parser.Match, byPID map[int][]parser.Sample, focus, opp int) Laning {
	l := Laning{Lane: DetectLane(byPID[focus]), LaneEnds: laneTo, Deaths: []parser.Death{}, Snaps: []LaneSnap{}}
	if opp >= 0 {
		l.OppLane = DetectLane(byPID[opp])
	}
	for _, t := range []float64{300, 600} {
		s, ok := sampleAt(byPID[focus], t)
		if !ok {
			continue
		}
		snap := LaneSnap{T: t, LH: s.LastHits, DN: s.Denies, NetWorth: s.NetWorth, XP: s.TotalXP, Level: s.Level}
		if opp >= 0 {
			if o, ok := sampleAt(byPID[opp], t); ok {
				snap.Opp = &LaneSnapValues{LH: o.LastHits, DN: o.Denies, NetWorth: o.NetWorth, XP: o.TotalXP, Level: o.Level}
			}
		}
		l.Snaps = append(l.Snaps, snap)
	}
	for _, d := range m.Deaths {
		if d.T > laneTo {
			continue
		}
		if d.VictimPID == focus {
			l.Deaths = append(l.Deaths, d)
		}
		if d.KillerPID == focus {
			l.Kills++
		}
		for _, a := range d.Assists {
			if a == focus {
				l.Assists++
			}
		}
	}
	return l
}

// ---- gold ----

// GoldSource is one slice of the earned-gold breakdown.
type GoldSource struct {
	Key    string `json:"key"` // creep, neutral, hero, building, passive, bounty, roshan, other
	Amount int    `json:"amount"`
}

// IdlePeriod is a stretch of alive time without farm or fight.
type IdlePeriod struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
	X    float64 `json:"x"` // position in the middle of the period
	Y    float64 `json:"y"`
	// Region is the map zone of that position (see Region).
	Region string `json:"region"`
}

// Gold is the Gold tab: sources, farm-over-time and idle periods.
type Gold struct {
	Sources   []GoldSource `json:"sources"`
	Creep     Line         `json:"creep"`   // cumulative lane-creep gold, aligned with Farm.T
	Neutral   Line         `json:"neutral"` // cumulative neutral gold
	GoldLost  int          `json:"gold_lost"`
	Idle      []IdlePeriod `json:"idle"`
	IdleTotal float64      `json:"idle_total"` // seconds
	Threshold float64      `json:"threshold"`
}

func goldSources(p parser.Player) []GoldSource {
	return []GoldSource{
		{"creep", p.GoldCreep}, {"neutral", p.GoldNeutral}, {"hero", p.GoldHero}, {"building", p.GoldBuilding},
		{"passive", p.GoldPassive}, {"bounty", p.GoldBounty}, {"roshan", p.GoldRoshan}, {"other", p.GoldOther},
	}
}

// IdlePeriods finds alive stretches of at least threshold seconds in which
// the player neither farmed (last hits, denies, neutral gold) nor fought or
// hit anything (damage dealt or taken).
func IdlePeriods(ss []parser.Sample, threshold float64) []IdlePeriod {
	out := []IdlePeriod{}
	start := -1
	flush := func(end int) {
		if start < 0 || end <= start {
			start = -1
			return
		}
		if ss[end].T-ss[start].T >= threshold {
			mid := ss[(start+end)/2]
			out = append(out, IdlePeriod{From: ss[start].T, To: ss[end].T, X: mid.X, Y: mid.Y, Region: Region(mid.X, mid.Y)})
		}
		start = -1
	}
	for i := 1; i < len(ss); i++ {
		prev, cur := ss[i-1], ss[i]
		if cur.T < 0 || !cur.Alive || !prev.Alive {
			flush(i - 1)
			continue
		}
		active := cur.LastHits > prev.LastHits || cur.Denies > prev.Denies || cur.GoldNeutral > prev.GoldNeutral ||
			cur.DmgOut > prev.DmgOut || cur.DmgIn > prev.DmgIn
		if active {
			flush(i - 1)
			continue
		}
		if start < 0 {
			start = i - 1
		}
	}
	flush(len(ss) - 1)
	return out
}

func buildGold(p parser.Player, ss []parser.Sample, axis []float64, opt Options) Gold {
	g := Gold{Sources: goldSources(p), GoldLost: p.GoldLost, Threshold: opt.IdleThreshold}
	index := map[float64]int{}
	for i, t := range axis {
		index[t] = i
	}
	g.Creep, g.Neutral = make(Line, len(axis)), make(Line, len(axis))
	for _, s := range ss {
		if i, ok := index[s.T]; ok {
			c, n := float64(s.GoldCreep), float64(s.GoldNeutral)
			g.Creep[i], g.Neutral[i] = &c, &n
		}
	}
	g.Idle = IdlePeriods(ss, opt.IdleThreshold)
	for _, p := range g.Idle {
		g.IdleTotal += p.To - p.From
	}
	return g
}

// ---- items ----

// ItemTiming is a purchase with the player's usual timing of the same item
// on the same hero (other matches, first purchase per match).
type ItemTiming struct {
	T     float64  `json:"t"`
	Item  string   `json:"item"`
	Avg   *float64 `json:"avg"`   // nil if never bought in other matches
	N     int      `json:"n"`     // matches the average is over
	Diff  *float64 `json:"diff"`  // T - Avg, first purchase only
	First bool     `json:"first"` // first purchase of this item in the match
}

func buildItems(m *parser.Match, focus int, avg map[string]ItemAvg) []ItemTiming {
	out := []ItemTiming{}
	seen := map[string]bool{}
	for _, p := range m.Purchases {
		if p.PlayerID != focus {
			continue
		}
		it := ItemTiming{T: p.T, Item: p.Item, First: !seen[p.Item]}
		seen[p.Item] = true
		if a, ok := avg[p.Item]; ok && a.N > 0 {
			v := a.Avg
			it.Avg, it.N = &v, a.N
			if it.First {
				d := p.T - v
				it.Diff = &d
			}
		}
		out = append(out, it)
	}
	return out
}

// ItemAvg is the average first-purchase time of an item.
type ItemAvg struct {
	Avg float64 `json:"avg"`
	N   int     `json:"n"`
}

// ---- fights ----

// Amount is a labelled total.
type Amount struct {
	Key    string `json:"key"` // hero npc name, unit category, inflictor or source kind
	Amount int    `json:"amount"`
	Count  int    `json:"count"`
}

// Fights is the Fights tab.
type Fights struct {
	TeamKills    int      `json:"team_kills"`
	KP           float64  `json:"kp"` // (kills + assists) / team kills
	DmgOutTotal  int      `json:"dmg_out_total"`
	DmgOutHeroes []Amount `json:"dmg_out_heroes"` // by enemy hero
	DmgOutUnits  []Amount `json:"dmg_out_units"`  // by unit category (creep, building, ...)
	DmgOutSource []Amount `json:"dmg_out_source"` // autoattack / ability / item, to heroes
	DmgOutByInfl []Amount `json:"dmg_out_by_inflictor"`
	DmgInTotal   int      `json:"dmg_in_total"`
	DmgInHeroes  []Amount `json:"dmg_in_heroes"` // by enemy hero
	DmgInUnits   []Amount `json:"dmg_in_units"`
	DmgInSource  []Amount `json:"dmg_in_source"` // from heroes
	DmgInByInfl  []Amount `json:"dmg_in_by_inflictor"`
	HealTotal    int      `json:"heal_total"`
	HealTargets  []Amount `json:"heal_targets"`
	BuildingDmg  int      `json:"building_dmg"`
	HeroDamage   int      `json:"hero_damage"` // scoreboard values
	TowerDamage  int      `json:"tower_damage"`
	Stuns        float64  `json:"stuns"`
}

func sourceKind(inflictor string) string {
	switch {
	case inflictor == "":
		return "autoattack"
	case strings.HasPrefix(inflictor, "item_"):
		return "item"
	}
	return "ability"
}

func isHero(npc string) bool { return strings.HasPrefix(npc, "npc_dota_hero_") }

type amounts map[string]*Amount

func (a amounts) add(key string, v, n int) {
	x := a[key]
	if x == nil {
		x = &Amount{Key: key}
		a[key] = x
	}
	x.Amount += v
	x.Count += n
}

func (a amounts) sorted() []Amount {
	out := make([]Amount, 0, len(a))
	for _, x := range a {
		out = append(out, *x)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func buildFights(m *parser.Match, me parser.Player) Fights {
	f := Fights{HeroDamage: me.HeroDamage, TowerDamage: me.TowerDamage, Stuns: me.Stuns}
	for _, p := range m.Players {
		if p.Team == me.Team {
			f.TeamKills += p.Kills
		}
	}
	if f.TeamKills > 0 {
		f.KP = math.Min(1, float64(me.Kills+me.Assists)/float64(f.TeamKills))
	}
	outH, outU, outS, outI := amounts{}, amounts{}, amounts{}, amounts{}
	inH, inU, inS, inI := amounts{}, amounts{}, amounts{}, amounts{}
	heal := amounts{}
	for _, c := range m.Combat {
		if c.PlayerID != me.PlayerID {
			continue
		}
		switch c.Kind {
		case parser.DmgOut:
			f.DmgOutTotal += c.Amount
			if isHero(c.Other) {
				outH.add(c.Other, c.Amount, c.Count)
				outS.add(sourceKind(c.Inflictor), c.Amount, c.Count)
				outI.add(c.Inflictor, c.Amount, c.Count)
			} else {
				outU.add(c.Other, c.Amount, c.Count)
			}
			if c.Other == "building" {
				f.BuildingDmg += c.Amount
			}
		case parser.DmgIn:
			f.DmgInTotal += c.Amount
			if isHero(c.Other) {
				inH.add(c.Other, c.Amount, c.Count)
				inS.add(sourceKind(c.Inflictor), c.Amount, c.Count)
				inI.add(c.Inflictor, c.Amount, c.Count)
			} else {
				inU.add(c.Other, c.Amount, c.Count)
			}
		case parser.HealOut:
			f.HealTotal += c.Amount
			heal.add(c.Other, c.Amount, c.Count)
		}
	}
	f.DmgOutHeroes, f.DmgOutUnits, f.DmgOutSource, f.DmgOutByInfl = outH.sorted(), outU.sorted(), outS.sorted(), outI.sorted()
	f.DmgInHeroes, f.DmgInUnits, f.DmgInSource, f.DmgInByInfl = inH.sorted(), inU.sorted(), inS.sorted(), inI.sorted()
	f.HealTargets = heal.sorted()
	return f
}

// ---- abilities ----

// AbilityUse is the cast summary of one ability or active item.
type AbilityUse struct {
	Ability string `json:"ability"`
	IsItem  bool   `json:"is_item"`
	Casts   int    `json:"casts"`
	// Hits: casts followed by damage/debuff on an enemy hero within the hit
	// window (heuristic). HitRate is nil when the ability never touched an
	// enemy hero in this match (probably not an offensive spell).
	Hits    int       `json:"hits"`
	HitRate *float64  `json:"hit_rate"`
	Damage  int       `json:"damage"` // damage to heroes by this inflictor
	Times   []float64 `json:"times"`  // cast times
}

// Abilities is the Abilities tab.
type Abilities struct {
	Uses      []AbilityUse `json:"uses"`
	HitWindow float64      `json:"hit_window"`
}

func buildAbilities(m *parser.Match, focus int, opt Options) Abilities {
	hits := map[string][]float64{}
	for _, h := range m.Hits {
		if h.PlayerID == focus {
			hits[h.Ability] = append(hits[h.Ability], h.T)
		}
	}
	dmg := map[string]int{}
	for _, c := range m.Combat {
		if c.PlayerID == focus && c.Kind == parser.DmgOut && isHero(c.Other) && c.Inflictor != "" {
			dmg[c.Inflictor] += c.Amount
		}
	}
	uses := map[string]*AbilityUse{}
	var order []string
	for _, c := range m.Casts {
		if c.PlayerID != focus {
			continue
		}
		u := uses[c.Ability]
		if u == nil {
			u = &AbilityUse{Ability: c.Ability, IsItem: strings.HasPrefix(c.Ability, "item_"), Damage: dmg[c.Ability]}
			uses[c.Ability] = u
			order = append(order, c.Ability)
		}
		u.Casts++
		u.Times = append(u.Times, c.T)
		hs := hits[c.Ability]
		i := sort.SearchFloat64s(hs, c.T)
		if i < len(hs) && hs[i]-c.T <= opt.HitWindow {
			u.Hits++
		}
	}
	a := Abilities{HitWindow: opt.HitWindow, Uses: []AbilityUse{}}
	for _, k := range order {
		u := uses[k]
		if len(hits[k]) > 0 && u.Casts > 0 {
			r := float64(u.Hits) / float64(u.Casts)
			u.HitRate = &r
		}
		a.Uses = append(a.Uses, *u)
	}
	sort.SliceStable(a.Uses, func(i, j int) bool {
		if a.Uses[i].IsItem != a.Uses[j].IsItem {
			return !a.Uses[i].IsItem
		}
		return a.Uses[i].Casts > a.Uses[j].Casts
	})
	return a
}

// ---- vision & map ----

// Vision is the Vision & map tab.
type Vision struct {
	Placed     []parser.Ward `json:"placed"`    // wards placed by the player
	Destroyed  []parser.Ward `json:"destroyed"` // enemy wards the player destroyed
	ObsPlaced  int           `json:"obs_placed"`
	SenPlaced  int           `json:"sen_placed"`
	MyLost     int           `json:"my_lost"`      // own wards destroyed by enemies
	ObsAvgLife float64       `json:"obs_avg_life"` // seconds
	Stacks     []float64     `json:"stacks"`       // stack times
	Runes      []parser.Rune `json:"runes"`
	Heat       [][2]float64  `json:"heat"` // alive positions (one per series step)
	TimeShare  TimeShare     `json:"time_share"`
	Lane       string        `json:"lane"`
}

func buildVision(m *parser.Match, me parser.Player, ss []parser.Sample, lane string) Vision {
	v := Vision{Placed: []parser.Ward{}, Destroyed: []parser.Ward{}, Stacks: []float64{}, Runes: []parser.Rune{},
		Heat: [][2]float64{}, Lane: lane}
	var obsLife float64
	var obsN int
	for _, w := range m.Wards {
		if w.PlayerID == me.PlayerID {
			v.Placed = append(v.Placed, w)
			if w.Sentry {
				v.SenPlaced++
			} else {
				v.ObsPlaced++
				obsLife += w.End - w.T
				obsN++
			}
			if w.Destroyed && w.KillerPID >= 0 && teamOf(m.Players, w.KillerPID) != me.Team {
				v.MyLost++
			}
		}
		if w.Destroyed && w.KillerPID == me.PlayerID && w.Team != me.Team {
			v.Destroyed = append(v.Destroyed, w)
		}
	}
	if obsN > 0 {
		v.ObsAvgLife = obsLife / float64(obsN)
	}
	for _, s := range m.Stacks {
		if s.PlayerID == me.PlayerID {
			v.Stacks = append(v.Stacks, s.T)
		}
	}
	for _, r := range m.Runes {
		if r.PlayerID == me.PlayerID {
			v.Runes = append(v.Runes, r)
		}
	}
	for _, s := range ss {
		if s.Alive && s.T >= 0 {
			v.Heat = append(v.Heat, [2]float64{s.X, s.Y})
		}
	}
	v.TimeShare = ComputeTimeShare(ss, me.Team, lane)
	return v
}
