package stats

import (
	"math"
	"sort"

	"github.com/1rowvy/drstats/internal/constants"
	"github.com/1rowvy/drstats/internal/parser"
)

// ---- game flow ----

// Advantage is the Radiant minus Dire net worth on the Farm.T axis (nil where
// a team has no samples).
func advantage(players []parser.Player, byPID map[int][]parser.Sample, axis []float64) Line {
	out := make(Line, len(axis))
	index := map[float64]int{}
	for i, t := range axis {
		index[t] = i
	}
	sum := [4][]float64{}
	cnt := [4][]int{}
	for _, team := range []int{parser.TeamRadiant, parser.TeamDire} {
		sum[team] = make([]float64, len(axis))
		cnt[team] = make([]int, len(axis))
	}
	for _, p := range players {
		if p.Team != parser.TeamRadiant && p.Team != parser.TeamDire {
			continue
		}
		for _, s := range byPID[p.PlayerID] {
			if i, ok := index[s.T]; ok {
				sum[p.Team][i] += float64(s.NetWorth)
				cnt[p.Team][i]++
			}
		}
	}
	for i := range axis {
		if cnt[parser.TeamRadiant][i] > 0 && cnt[parser.TeamDire][i] > 0 {
			v := sum[parser.TeamRadiant][i] - sum[parser.TeamDire][i]
			out[i] = &v
		}
	}
	return out
}

// ObjectiveView is a destroyed building or a Roshan kill on the timeline.
type ObjectiveView struct {
	parser.Objective
	Tick uint32 `json:"tick"`
}

// ---- fights ----

// Fight detection thresholds.
const (
	fightBucketDmg  = 600.0 // hero-to-hero damage per 5 s bucket that counts as fighting
	fightGap        = 10.0  // seconds of calm that split two fights
	fightMinDmg     = 4000  // total damage for a fight without deaths to count
	fightInvolveDmg = 150   // dealt + taken by a player to count as a participant
)

// FightHero is one participant's damage in a fight.
type FightHero struct {
	PlayerID int `json:"pid"`
	Dealt    int `json:"dealt"`
	Taken    int `json:"taken"`
}

// Battle is one team fight / skirmish the focused player took part in.
type Battle struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Tick   uint32  `json:"tick"` // replay tick a few seconds before the start
	Region string  `json:"region"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	// Participants by side, from the focused player's point of view.
	Allies  int `json:"allies"`
	Enemies int `json:"enemies"`
	// Hero deaths in the fight: enemies killed, allies lost.
	Kills  int  `json:"kills"`
	Losses int  `json:"losses"`
	Won    bool `json:"won"`
	// GoldSwing is the change of the team net worth difference over the
	// fight, positive = good for the focused player's team.
	GoldSwing int `json:"gold_swing"`
	// Focused player.
	MyDamage  int         `json:"my_damage"`
	MyTaken   int         `json:"my_taken"`
	MyShare   float64     `json:"my_share"` // of the team's damage in the fight
	MyKills   int         `json:"my_kills"` // killing blows
	MyAssists int         `json:"my_assists"`
	MyDied    bool        `json:"my_died"`
	MySources []Amount    `json:"my_sources"` // my damage by inflictor
	Heroes    []FightHero `json:"heroes"`
	// Deaths are indexes into MatchView.Deaths.
	Deaths []int `json:"deaths"`
	// Play: who started, when I joined, key spells used / wasted.
	Play FightPlay `json:"play"`
}

type span struct{ from, to float64 }

// fightSpans groups damage buckets and deaths into fight windows.
func fightSpans(m *parser.Match) []span {
	perBucket := map[float64]int{}
	for _, b := range m.HeroDamage {
		perBucket[b.T] += b.Amount
	}
	var hot []float64
	for t, v := range perBucket {
		if float64(v) >= fightBucketDmg {
			hot = append(hot, t)
		}
	}
	for _, d := range m.Deaths {
		hot = append(hot, math.Floor(d.T/parser.DamageBucketStep)*parser.DamageBucketStep)
	}
	sort.Float64s(hot)
	var out []span
	for _, t := range hot {
		end := t + parser.DamageBucketStep
		if n := len(out); n > 0 && t-out[n-1].to <= fightGap {
			out[n-1].to = math.Max(out[n-1].to, end)
			continue
		}
		out = append(out, span{t, end})
	}
	return out
}

func buildBattles(m *parser.Match, players []parser.Player, byPID map[int][]parser.Sample, focus int, clock *parser.Clock,
	cds map[string]constants.Cooldown) []Battle {
	out := []Battle{}
	team := teamOf(players, focus)
	if team == 0 {
		return out
	}
	teamNW := func(t float64) (float64, bool) {
		var diff float64
		n := 0
		for _, p := range players {
			s, ok := sampleAt(byPID[p.PlayerID], t)
			if !ok {
				continue
			}
			n++
			if p.Team == team {
				diff += float64(s.NetWorth)
			} else {
				diff -= float64(s.NetWorth)
			}
		}
		return diff, n > 0
	}
	snap := func(t float64) float64 { return math.Round(t/parser.SeriesStep) * parser.SeriesStep }

	for _, sp := range fightSpans(m) {
		b := Battle{Start: sp.from, End: sp.to, Heroes: []FightHero{}, Deaths: []int{}, MySources: []Amount{}}
		dealt, taken := map[int]int{}, map[int]int{}
		mine := amounts{}
		var dmgTotal int
		for _, d := range m.HeroDamage {
			if d.T < sp.from || d.T >= sp.to {
				continue
			}
			dealt[d.PlayerID] += d.Amount
			taken[d.Target] += d.Amount
			dmgTotal += d.Amount
			if d.PlayerID == focus {
				mine.add(d.Inflictor, d.Amount, 1)
			}
		}
		var xs, ys []float64
		for i, d := range m.Deaths {
			if d.T < sp.from || d.T >= sp.to+1 {
				continue
			}
			b.Deaths = append(b.Deaths, i)
			if teamOf(players, d.VictimPID) == team {
				b.Losses++
			} else {
				b.Kills++
			}
			if d.VictimPID == focus {
				b.MyDied = true
			}
			if d.KillerPID == focus {
				b.MyKills++
			}
			for _, a := range d.Assists {
				if a == focus {
					b.MyAssists++
				}
			}
			if d.HasPos {
				xs, ys = append(xs, d.X), append(ys, d.Y)
			}
		}
		if len(b.Deaths) == 0 && dmgTotal < fightMinDmg {
			continue
		}
		involved := func(pid int) bool { return dealt[pid]+taken[pid] >= fightInvolveDmg }
		if !involved(focus) && !b.MyDied && b.MyKills == 0 && b.MyAssists == 0 {
			continue
		}
		var teamDealt int
		for _, p := range players {
			if dealt[p.PlayerID]+taken[p.PlayerID] == 0 {
				continue
			}
			b.Heroes = append(b.Heroes, FightHero{PlayerID: p.PlayerID, Dealt: dealt[p.PlayerID], Taken: taken[p.PlayerID]})
			if p.Team == team {
				teamDealt += dealt[p.PlayerID]
			}
			if involved(p.PlayerID) {
				if p.Team == team {
					b.Allies++
				} else {
					b.Enemies++
				}
			}
		}
		sort.Slice(b.Heroes, func(i, j int) bool { return b.Heroes[i].Dealt > b.Heroes[j].Dealt })
		b.MyDamage, b.MyTaken = dealt[focus], taken[focus]
		if teamDealt > 0 {
			b.MyShare = float64(b.MyDamage) / float64(teamDealt)
		}
		b.MySources = mine.sorted()
		// Position: the deaths' centroid, else where I was at the start.
		if len(xs) > 0 {
			for i := range xs {
				b.X += xs[i] / float64(len(xs))
				b.Y += ys[i] / float64(len(ys))
			}
		} else if s, ok := sampleAt(byPID[focus], snap(sp.from)); ok {
			b.X, b.Y = s.X, s.Y
		}
		b.Region = Region(b.X, b.Y)
		b.Play = fightPlay(m, &b, sp.from, sp.to, players, byPID, focus, cds)
		before, ok1 := teamNW(snap(sp.from - parser.SeriesStep))
		after, ok2 := teamNW(snap(sp.to + 2*parser.SeriesStep))
		if ok1 && ok2 {
			b.GoldSwing = int(math.Round(after - before))
		}
		b.Won = b.Kills > b.Losses || (b.Kills == b.Losses && b.GoldSwing > 0)
		if clock != nil {
			b.Tick = clock.Tick(math.Max(sp.from-5, -90))
		}
		out = append(out, b)
	}
	return out
}

// ---- grade ----

// Grade is an overall mark of the focused player's game: percentile of the
// key metrics against their own other matches (same hero when there are
// enough), plus a bonus for the win.
type Grade struct {
	Score  int    `json:"score"` // 0..100
	Letter string `json:"letter"`
	// Basis is how many own matches the percentiles are taken over and
	// whether they are on the same hero.
	Basis  int  `json:"basis"`
	OnHero bool `json:"on_hero"`
}

// Grade parameters.
const (
	gradeMinBasis  = 5  // fewer reference matches: no grade
	gradeHeroBasis = 8  // on-hero reference used from this many matches
	gradeWinBonus  = 15 // points out of 100 that the result is worth
)

var gradeMetrics = map[string]float64{
	"kda": 1.5, "gpm": 1.2, "xpm": 0.8, "hero_dmg_pm": 1, "kp": 1, "deaths": 1, "lh10": 0.8, "tower_dmg": 0.5,
}

var gradeLetters = []struct {
	min    int
	letter string
}{
	{90, "A+"}, {80, "A"}, {72, "A−"}, {64, "B+"}, {56, "B"}, {48, "B−"}, {40, "C+"}, {32, "C"}, {24, "C−"}, {0, "D"},
}

func letterOf(score int) string {
	for _, g := range gradeLetters {
		if score >= g.min {
			return g.letter
		}
	}
	return "D"
}

// Grades marks every game against the others (oldest first input).
func Grades(games []Game) map[int64]Grade {
	out := map[int64]Grade{}
	byHero := map[int][]int{}
	for i := range games {
		byHero[games[i].HeroID] = append(byHero[games[i].HeroID], i)
	}
	all := make([]int, len(games))
	for i := range all {
		all[i] = i
	}
	defs := map[string]metricDef{}
	for _, m := range metrics {
		defs[m.Key] = m
	}
	for i := range games {
		g := &games[i]
		ref, onHero := all, false
		if len(byHero[g.HeroID])-1 >= gradeHeroBasis {
			ref, onHero = byHero[g.HeroID], true
		}
		if len(ref)-1 < gradeMinBasis {
			continue
		}
		var sum, weight float64
		for key, w := range gradeMetrics {
			m := defs[key]
			v, ok := m.Value(g)
			if !ok {
				continue
			}
			var below, equal, n float64
			for _, j := range ref {
				if j == i {
					continue
				}
				o, ok := m.Value(&games[j])
				if !ok {
					continue
				}
				n++
				switch {
				case o == v:
					equal++
				case (o < v) == m.HigherBetter:
					below++
				}
			}
			if n < gradeMinBasis {
				continue
			}
			sum += w * (below + equal/2) / n
			weight += w
		}
		if weight == 0 {
			continue
		}
		score := (sum / weight) * (100 - gradeWinBonus)
		if g.Win {
			score += gradeWinBonus
		}
		s := int(math.Round(score))
		out[g.MatchID] = Grade{Score: s, Letter: letterOf(s), Basis: len(ref) - 1, OnHero: onHero}
	}
	return out
}
