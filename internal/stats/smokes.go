package stats

import (
	"sort"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

// Smoke analysis: heroes smoked by one use are grouped, and the smoke is
// judged by the hero kills on both sides while it lasted and shortly after it
// broke (the gank itself starts when the buff breaks).
const (
	smokeGroupGap = 3.0  // buffs from the same user within this many seconds = one smoke
	smokeAfter    = 15.0 // kills this long after the buff ended still count
	smokeObjAfter = 40.0 // a building / Roshan taken this long after still counts
)

// Smoke results, from the smoking team's side.
const (
	SmokeKill      = "kill"      // more enemy heroes died than own
	SmokeTrade     = "trade"     // as many as own
	SmokeLost      = "lost"      // more own heroes died
	SmokeObjective = "objective" // no kills, but a building or Roshan fell
	SmokeNothing   = "nothing"
)

// SmokeView is one use of Smoke of Deceit.
type SmokeView struct {
	T       float64 `json:"t"`
	End     float64 `json:"end"`   // the last buff of the group ended (broke or expired)
	Break   float64 `json:"break"` // the first buff ended: the smoke was found or broken
	Tick    uint32  `json:"tick"`  // replay tick a little before the use (demo_gototick)
	Team    int     `json:"team"`
	User    int     `json:"user"` // -1 if unknown
	Members []int   `json:"members"`
	Mine    bool    `json:"mine"` // the focused player was under it
	Enemy   bool    `json:"enemy"`
	// Kills are enemy heroes killed by (or with an assist of) a smoked hero
	// during the smoke, Lost smoked heroes that died (player ids, in order).
	Kills []int `json:"kills"`
	Lost  []int `json:"lost"`
	// Involved: the focused player got a kill or an assist on Kills; Died:
	// the focused player is among the dead (own or enemy smoke).
	Involved  bool   `json:"involved"`
	Died      bool   `json:"died"`
	Objective string `json:"objective"` // npc name of a building / Roshan the smoking team took after it
	Result    string `json:"result"`
	// Region where the smoke broke (position of the first member whose buff
	// ended); "" without a position.
	Region string `json:"region"`
}

// analyzeSmokes groups smoked heroes into smokes and judges them. teams maps
// player id to team; focus may be -1.
func analyzeSmokes(smokes []parser.Smoke, deaths []parser.Death, objs []parser.Objective, teams map[int]int, focus int) []SmokeView {
	sorted := append([]parser.Smoke(nil), smokes...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].T < sorted[j].T })
	out := []SmokeView{}
	for _, s := range sorted {
		team := teams[s.PlayerID]
		gi := -1
		for i := len(out) - 1; i >= 0 && s.T-out[i].T <= smokeGroupGap; i-- {
			if out[i].Team == team && out[i].User == s.UserPID {
				gi = i
				break
			}
		}
		if gi < 0 {
			gi = len(out)
			out = append(out, SmokeView{T: s.T, End: s.End, Break: s.End, Team: team, User: s.UserPID,
				Members: []int{}, Kills: []int{}, Lost: []int{}})
		}
		v := &out[gi]
		v.Members = append(v.Members, s.PlayerID)
		v.End = max(v.End, s.End)
		v.Break = min(v.Break, s.End)
	}
	focusTeam := teams[focus]
	for i := range out {
		v := &out[i]
		for _, m := range v.Members {
			if m == focus {
				v.Mine = true
			}
		}
		v.Enemy = focus >= 0 && v.Team != focusTeam
		member := map[int]bool{}
		for _, m := range v.Members {
			member[m] = true
		}
		to := v.End + smokeAfter
		for _, d := range deaths {
			if d.T < v.T || d.T > to {
				continue
			}
			if teams[d.VictimPID] == v.Team {
				if member[d.VictimPID] {
					v.Lost = append(v.Lost, d.VictimPID)
				}
			} else {
				by := member[d.KillerPID]
				for _, a := range d.Assists {
					by = by || member[a]
				}
				if !by {
					continue
				}
				v.Kills = append(v.Kills, d.VictimPID)
				if d.KillerPID == focus && focus >= 0 {
					v.Involved = true
				}
				for _, a := range d.Assists {
					if a == focus && focus >= 0 {
						v.Involved = true
					}
				}
			}
			if d.VictimPID == focus && focus >= 0 {
				v.Died = true
			}
		}
		for _, o := range objs {
			taker := o.Team != v.Team && o.Team != 0
			if o.Kind == "roshan" {
				taker = o.KillerPID >= 0 && teams[o.KillerPID] == v.Team
			}
			if taker && o.T >= v.T && o.T <= v.End+smokeObjAfter && o.Kind != "fort" {
				v.Objective = o.Key
				break
			}
		}
		switch k, l := len(v.Kills), len(v.Lost); {
		case k > l:
			v.Result = SmokeKill
		case k > 0 && k == l:
			v.Result = SmokeTrade
		case l > k:
			v.Result = SmokeLost
		case v.Objective != "":
			v.Result = SmokeObjective
		default:
			v.Result = SmokeNothing
		}
	}
	return out
}

// smokeRegions fills the region where each smoke broke from the series.
func smokeRegions(views []SmokeView, smokes []parser.Smoke, byPID map[int][]parser.Sample) {
	for i := range views {
		v := &views[i]
		pid := -1
		for _, s := range smokes {
			if s.End == v.Break && s.T >= v.T && s.T <= v.T+smokeGroupGap {
				pid = s.PlayerID
				break
			}
		}
		if ss := byPID[pid]; len(ss) > 0 {
			j := sort.Search(len(ss), func(j int) bool { return ss[j].T > v.Break })
			if j > 0 {
				v.Region = Region(ss[j-1].X, ss[j-1].Y)
			}
		}
	}
}

// SmokeStats summarise smokes over the filtered games with smoke data
// (replays parsed by parser v4+).
type SmokeStats struct {
	Games   int     `json:"games"`
	Smokes  int     `json:"smokes"` // own-team smokes you were under
	PerGame float64 `json:"per_game"`
	Used    int     `json:"used"` // smokes you used yourself
	// Results of your smokes.
	Kill      int `json:"kill"`
	Trade     int `json:"trade"`
	Lost      int `json:"lost"`
	Objective int `json:"objective"`
	Nothing   int `json:"nothing"`
	Involved  int `json:"involved"` // you got a kill / assist
	Died      int `json:"died"`     // you died in it
	// Enemy smokes and your deaths in them (ganked by a smoke).
	Enemy       int `json:"enemy"`
	Caught      int `json:"caught"`
	CaughtGames int `json:"caught_games"`
	// Winrate of games with at least one successful own smoke vs games with
	// smokes but none successful (nil below 3 games on a side).
	WinGood *float64 `json:"win_good"`
	WinBad  *float64 `json:"win_bad"`
	NGood   int      `json:"n_good"`
	NBad    int      `json:"n_bad"`
}

const smokeMinSide = 3

func smokeStats(games []Game, sg map[int64]store.SmokeGame) SmokeStats {
	var st SmokeStats
	var goodW, goodN, badW, badN int
	for i := range games {
		g := &games[i]
		x, ok := sg[g.MatchID]
		if !ok || g.OpenDota {
			continue
		}
		st.Games++
		good, had, caught := false, false, false
		for _, v := range analyzeSmokes(x.Smokes, x.Deaths, x.Objectives, x.Teams, x.PlayerID) {
			if v.Enemy {
				st.Enemy++
				if v.Died {
					st.Caught++
					caught = true
				}
				continue
			}
			if !v.Mine {
				continue
			}
			had = true
			st.Smokes++
			if v.User == x.PlayerID {
				st.Used++
			}
			switch v.Result {
			case SmokeKill:
				st.Kill++
				good = true
			case SmokeTrade:
				st.Trade++
			case SmokeLost:
				st.Lost++
			case SmokeObjective:
				st.Objective++
				good = true
			default:
				st.Nothing++
			}
			if v.Involved {
				st.Involved++
			}
			if v.Died {
				st.Died++
			}
		}
		if caught {
			st.CaughtGames++
		}
		switch {
		case good:
			goodN++
			if g.Win {
				goodW++
			}
		case had:
			badN++
			if g.Win {
				badW++
			}
		}
	}
	if st.Games > 0 {
		st.PerGame = float64(st.Smokes) / float64(st.Games)
	}
	st.NGood, st.NBad = goodN, badN
	if goodN >= smokeMinSide && badN >= smokeMinSide {
		a, b := float64(goodW)/float64(goodN), float64(badW)/float64(badN)
		st.WinGood, st.WinBad = &a, &b
	}
	return st
}
