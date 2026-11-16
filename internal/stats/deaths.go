package stats

import (
	"math"
	"sort"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

// Death classification thresholds.
const (
	allyRadius    = 1600.0 // an allied hero this close means the victim was not alone
	deathRichGold = 1500   // unspent gold at death worth a note
	earlyLaneTime = 600.0  // before this a death in the own lane is laning, not a misplay
)

// DeathCtx describes the surroundings of a death.
type DeathCtx struct {
	Region string `json:"region"` // map zone key, see Region
	Vision bool   `json:"vision"` // an allied observer ward saw the spot
	Tick   uint32 `json:"tick"`   // replay tick a few seconds before the death
	// Allies alive within allyRadius; Enemies are the heroes credited with
	// the kill (killing blow + assists).
	Allies  int  `json:"allies"`
	Enemies int  `json:"enemies"`
	Alone   bool `json:"alone"`
	Deep    bool `json:"deep"` // on the enemy half, outside the river
	Gold    int  `json:"gold"` // unspent gold right before the death, -1 unknown
	// Avoidable: alone and either blind, deep in enemy land or caught by
	// several heroes — a death map awareness or positioning would have saved.
	Avoidable bool `json:"avoidable"`
}

// obsRadius is the observer ward vision radius (day), world units.
const obsRadius = 1400.0

// classifyDeath judges one death. heroes are the latest samples of the
// heroes right before it (victim included), teams maps player id to team.
func classifyDeath(d parser.Death, team int, heroes []parser.Sample, teams map[int]int, wards []parser.Ward) DeathCtx {
	c := DeathCtx{Gold: -1}
	killers := map[int]bool{}
	if d.KillerPID >= 0 && teams[d.KillerPID] != team {
		killers[d.KillerPID] = true
	}
	for _, a := range d.Assists {
		if a >= 0 && teams[a] != team {
			killers[a] = true
		}
	}
	c.Enemies = len(killers)
	x, y, has := d.X, d.Y, d.HasPos
	for _, s := range heroes {
		if s.PlayerID == d.VictimPID {
			c.Gold = s.Gold
			if !has {
				x, y, has = s.X, s.Y, true
			}
		}
	}
	if !has {
		return c
	}
	c.Region = Region(x, y)
	for _, w := range wards {
		if !w.Sentry && w.Team == team && w.T <= d.T && d.T <= w.End && math.Hypot(w.X-x, w.Y-y) <= obsRadius {
			c.Vision = true
			break
		}
	}
	known := false
	for _, s := range heroes {
		if s.PlayerID == d.VictimPID || teams[s.PlayerID] != team {
			continue
		}
		known = true
		if s.Alive && math.Hypot(s.X-x, s.Y-y) <= allyRadius {
			c.Allies++
		}
	}
	c.Alone = known && c.Allies == 0
	c.Deep = enemyHalf(team, x, y) && math.Abs(x+y) > 1500 && !inRiver(x, y)
	laning := d.T < earlyLaneTime && laneAt(x, y) != "" && !enemyHalf(team, x, y)
	outnumbered := c.Enemies >= 2 && c.Enemies > c.Allies+1
	c.Avoidable = c.Alone && (c.Deep || outnumbered || (!c.Vision && !laning))
	return c
}

// deathContext classifies every death of a match.
func deathContext(players []parser.Player, deaths []parser.Death, wards []parser.Ward, byPID map[int][]parser.Sample) []DeathCtx {
	teams := map[int]int{}
	for _, p := range players {
		teams[p.PlayerID] = p.Team
	}
	out := make([]DeathCtx, len(deaths))
	for i, d := range deaths {
		var heroes []parser.Sample
		for pid, ss := range byPID {
			// Last sample at or before the death, within one step.
			j := sort.Search(len(ss), func(k int) bool { return ss[k].T > d.T }) - 1
			if j >= 0 && d.T-ss[j].T < parser.SeriesStep {
				s := ss[j]
				s.PlayerID = pid
				heroes = append(heroes, s)
			}
		}
		out[i] = classifyDeath(d, teams[d.VictimPID], heroes, teams, wards)
	}
	return out
}

// classifiedDeath is a death of the account with its context.
type classifiedDeath struct {
	MatchID  int64
	T        float64
	GoldLost int
	Ctx      DeathCtx
}

// classifyScenes judges the stored deaths of the account, grouped by match.
func classifyScenes(scenes []store.DeathScene) map[int64][]classifiedDeath {
	out := map[int64][]classifiedDeath{}
	for _, sc := range scenes {
		c := classifyDeath(sc.Death, sc.Team, sc.Heroes, sc.Teams, sc.Wards)
		out[sc.MatchID] = append(out[sc.MatchID], classifiedDeath{MatchID: sc.MatchID, T: sc.Death.T, GoldLost: sc.Death.GoldLost, Ctx: c})
	}
	return out
}

// DeathStats is the dashboard card about avoidable deaths (replays only).
type DeathStats struct {
	Games     int     `json:"games"`
	Deaths    int     `json:"deaths"`
	Avoidable int     `json:"avoidable"`
	PerGame   float64 `json:"per_game"`  // avoidable deaths per game
	GoldLost  float64 `json:"gold_lost"` // gold lost on avoidable deaths, per game
	// Why the avoidable ones were avoidable (a death can count in several).
	Blind       int        `json:"blind"`       // alone without vision
	Deep        int        `json:"deep"`        // alone on the enemy half
	Outnumbered int        `json:"outnumbered"` // alone against 2+ heroes
	Rich        int        `json:"rich"`        // any death with ≥ deathRichGold unspent
	Zones       []IdleZone `json:"zones"`       // where the avoidable ones happen, count in Sec; most first
	// Winrate of games with at most one avoidable death vs more.
	Clean  Bucket  `json:"clean"`
	Messy  Bucket  `json:"messy"`
	Recent float64 `json:"recent"` // avoidable per game, last 10 games
	Before float64 `json:"before"` // avoidable per game, the games before
}

func deathStats(games []Game, byMatch map[int64][]classifiedDeath) DeathStats {
	ds := DeathStats{Zones: []IdleZone{}, Clean: Bucket{Key: "clean"}, Messy: Bucket{Key: "messy"}}
	zones := map[string]float64{}
	var per []float64
	for i := range games {
		g := &games[i]
		if g.OpenDota {
			continue
		}
		ds.Games++
		n := 0
		for _, d := range byMatch[g.MatchID] {
			ds.Deaths++
			if d.Ctx.Gold >= deathRichGold {
				ds.Rich++
			}
			if !d.Ctx.Avoidable {
				continue
			}
			n++
			ds.GoldLost += float64(d.GoldLost)
			if !d.Ctx.Vision {
				ds.Blind++
			}
			if d.Ctx.Deep {
				ds.Deep++
			}
			if d.Ctx.Enemies >= 2 {
				ds.Outnumbered++
			}
			if d.Ctx.Region != "" {
				zones[d.Ctx.Region]++
			}
		}
		ds.Avoidable += n
		per = append(per, float64(n))
		if n <= 1 {
			addBucket(&ds.Clean, g)
		} else {
			addBucket(&ds.Messy, g)
		}
	}
	finishBucket(&ds.Clean)
	finishBucket(&ds.Messy)
	if ds.Games == 0 {
		return ds
	}
	ds.PerGame = float64(ds.Avoidable) / float64(ds.Games)
	ds.GoldLost /= float64(ds.Games)
	for r, n := range zones {
		ds.Zones = append(ds.Zones, IdleZone{Region: r, Sec: n})
	}
	sort.Slice(ds.Zones, func(i, j int) bool { return ds.Zones[i].Sec > ds.Zones[j].Sec })
	if len(ds.Zones) > 4 {
		ds.Zones = ds.Zones[:4]
	}
	if len(per) > 10 {
		ds.Recent = meanOf(per[len(per)-10:])
		ds.Before = meanOf(per[:len(per)-10])
	}
	return ds
}

// avoidableCounts is the number of avoidable deaths per match.
func avoidableCounts(byMatch map[int64][]classifiedDeath) map[int64]int {
	out := map[int64]int{}
	for id, ds := range byMatch {
		for _, d := range ds {
			if d.Ctx.Avoidable {
				out[id]++
			}
		}
	}
	return out
}
