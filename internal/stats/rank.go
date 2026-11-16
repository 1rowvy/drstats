package stats

import (
	"math"
	"sort"
	"time"

	"github.com/1rowvy/drstats/internal/store"
)

// LobbyRanked is the lobby type of ranked matchmaking.
const LobbyRanked = 7

// MMRPerGame is the assumed rating change of one ranked game. The real
// change varies a little (calibration uncertainty), so the curve is an
// estimate and is pinned to the medals actually seen.
const MMRPerGame = 25

// rankMinGames is how many games a hero / lane / party bucket needs to be listed.
const rankMinGames = 3

// TierMMR is the approximate MMR in the middle of a medal star
// (rank tier 54 = Legend 4; 80 = Immortal). 0 for an unknown tier.
func TierMMR(tier int) float64 {
	medal, star := tier/10, tier%10
	switch {
	case medal >= 8:
		return 5620 + 100
	case medal < 1 || star < 1:
		return 0
	}
	if star > 5 {
		star = 5
	}
	if medal == 7 { // Divine stars are 200 wide
		return 4620 + float64(star-1)*200 + 100
	}
	return float64((medal-1)*5+star-1)*154 + 77
}

// MMRTier is the medal (rank tier) an MMR falls into.
func MMRTier(mmr float64) int {
	switch {
	case mmr >= 5620:
		return 80
	case mmr >= 4620:
		return 70 + min(5, int((mmr-4620)/200)+1)
	case mmr < 0:
		return 11
	}
	k := int(mmr / 154)
	return (k/5+1)*10 + k%5 + 1
}

// nextTierMMR is where the star after tier starts (0 for Immortal).
func nextTierMMR(tier int) float64 {
	medal, star := tier/10, tier%10
	switch {
	case medal >= 8:
		return 0
	case medal == 7:
		if star >= 5 {
			return 5620
		}
		return 4620 + float64(star)*200
	}
	return float64((medal-1)*5+star) * 154
}

// RankPoint is one ranked game on the MMR curve.
type RankPoint struct {
	MatchID  int64   `json:"match_id"`
	EndTime  int64   `json:"end_time"`
	Win      bool    `json:"win"`
	HeroName string  `json:"hero_name"`
	OpenDota bool    `json:"opendota"` // no replay: opens on OpenDota
	MMR      float64 `json:"mmr"`      // estimated, after the game
}

// RankMark is a medal actually seen: in a replay / OpenDota match or in the
// OpenDota profile. Index is the curve point it belongs to.
type RankMark struct {
	Index   int     `json:"index"`
	T       int64   `json:"t"`
	Tier    int     `json:"tier"`
	MMR     float64 `json:"mmr"`     // middle of that star
	Profile bool    `json:"profile"` // from the profile (else from a match)
}

// RankBucket is the rating won or lost in a group of ranked games.
type RankBucket struct {
	Key   string `json:"key"`
	Games int    `json:"games"`
	Wins  int    `json:"wins"`
	Net   int    `json:"net"` // estimated MMR
}

func (b *RankBucket) add(win bool) {
	b.Games++
	if win {
		b.Wins++
		b.Net += MMRPerGame
	} else {
		b.Net -= MMRPerGame
	}
}

// RankView is the Rank page.
type RankView struct {
	Unknown bool `json:"unknown"` // no Steam ID configured
	Days    int  `json:"days"`    // period, 0 = all time
	// Anchored is true when the curve is pinned to a medal seen; otherwise
	// MMR values are relative (the curve starts at 0).
	Anchored bool        `json:"anchored"`
	Points   []RankPoint `json:"points"` // ranked games in the period, oldest first
	Marks    []RankMark  `json:"marks"`  // medals seen in the period
	Total    int         `json:"total"`  // ranked games, all time

	Tier        int     `json:"tier"`        // last medal seen (0 = never)
	TierAt      int64   `json:"tier_at"`     // when, unix
	Leaderboard int     `json:"leaderboard"` // Immortal place, from the profile
	PeakTier    int     `json:"peak_tier"`   // best medal seen, all time
	MMR         float64 `json:"mmr"`         // current estimate
	EstTier     int     `json:"est_tier"`    // medal of the current estimate
	NextAt      float64 `json:"next_at"`     // MMR of the next star (0 = Immortal)

	Games int `json:"games"` // in the period
	Wins  int `json:"wins"`
	Net   int `json:"net"`
	Net7  int `json:"net7"`  // last 7 days
	Net30 int `json:"net30"` // last 30 days

	Peak     float64      `json:"peak"` // best estimate in the period
	PeakAt   int64        `json:"peak_at"`
	Low      float64      `json:"low"`
	BestRun  int          `json:"best_run"`  // longest win streak in the period
	WorstRun int          `json:"worst_run"` // longest loss streak
	Streak   int          `json:"streak"`    // current streak, > 0 wins, < 0 losses
	ByHero   []RankBucket `json:"by_hero"`   // key: hero npc name; most net first
	ByLane   []RankBucket `json:"by_lane"`   // key: safe / mid / off / jungle / roam
	ByParty  []RankBucket `json:"by_party"`  // key: solo / party
	ByPeriod []RankBucket `json:"by_period"` // key: "2026-11-09" (week) or "2026-11" (month), oldest first
	Weekly   bool         `json:"weekly"`    // ByPeriod is by week
}

// laneRole maps the lane to safe / off for the side the player was on.
func laneRole(g *Game) string {
	switch g.Lane {
	case LaneMid, LaneJungle, LaneRoam:
		return g.Lane
	case LaneBot:
		if g.Team == teamRadiant {
			return "safe"
		}
		return "off"
	case LaneTop:
		if g.Team == teamRadiant {
			return "off"
		}
		return "safe"
	}
	return ""
}

// BuildRank estimates the rating curve from ranked wins and losses (replays
// and OpenDota history) and analyses the period of the last days (0 = all).
func BuildRank(st *store.Store, accountID uint32, days int, heroByID map[int]string, now time.Time) (*RankView, error) {
	v := &RankView{Unknown: accountID == 0, Days: days, Points: []RankPoint{}, Marks: []RankMark{},
		ByHero: []RankBucket{}, ByLane: []RankBucket{}, ByParty: []RankBucket{}, ByPeriod: []RankBucket{}}
	if accountID == 0 {
		return v, nil
	}
	all, err := loadGames(st, accountID)
	if err != nil {
		return nil, err
	}
	hist, err := st.History(accountID)
	if err != nil {
		return nil, err
	}
	all = mergeHistory(all, hist, HeroNamer(heroByID), true)
	profile, err := st.Ranks(accountID)
	if err != nil {
		return nil, err
	}
	var games []Game
	for _, g := range all {
		if g.LobbyType == LobbyRanked {
			games = append(games, g)
		}
	}
	v.Total = len(games)
	buildRank(v, games, profile, now)
	return v, nil
}

func buildRank(v *RankView, games []Game, profile []store.RankSeen, now time.Time) {
	// Cumulative rating change after each game, starting at 0.
	cum := make([]float64, len(games))
	c := 0.0
	for i, g := range games {
		if g.Win {
			c += MMRPerGame
		} else {
			c -= MMRPerGame
		}
		cum[i] = c
	}
	// before(t): games finished at or before t.
	before := func(t int64) int {
		return sort.Search(len(games), func(i int) bool { return games[i].EndTime > t })
	}
	cumAt := func(n int) float64 { // after the first n games
		if n == 0 {
			return 0
		}
		return cum[n-1]
	}

	// Medals seen, oldest first. A match medal is the one before that game.
	type seen struct {
		mark RankMark
		base float64 // cumulative change at that moment
	}
	var obs []seen
	for i, g := range games {
		if g.RankTier > 0 {
			obs = append(obs, seen{RankMark{Index: i, T: g.EndTime - int64(g.Duration), Tier: g.RankTier}, cumAt(i)})
		}
	}
	for _, p := range profile {
		n := before(p.T)
		obs = append(obs, seen{RankMark{Index: max(0, n-1), T: p.T, Tier: p.RankTier, Profile: true}, cumAt(n)})
		v.Leaderboard = p.LeaderboardRank
	}
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].mark.T < obs[j].mark.T })
	offset := 0.0
	if len(obs) > 0 {
		last := obs[len(obs)-1]
		v.Anchored = true
		v.Tier, v.TierAt = last.mark.Tier, last.mark.T
		if !last.mark.Profile {
			v.Leaderboard = 0
		}
		offset = TierMMR(last.mark.Tier) - last.base
		for _, o := range obs {
			v.PeakTier = max(v.PeakTier, o.mark.Tier)
		}
	}
	v.MMR = offset + cumAt(len(games))
	if v.Anchored {
		v.EstTier = MMRTier(v.MMR)
		v.NextAt = nextTierMMR(v.EstTier)
	}

	// Period.
	from := 0
	if v.Days > 0 {
		from = before(now.AddDate(0, 0, -v.Days).Unix())
	}
	for _, o := range obs {
		if o.mark.Index >= from && (v.Days == 0 || o.mark.T >= now.AddDate(0, 0, -v.Days).Unix()) {
			m := o.mark
			m.Index -= from
			m.MMR = TierMMR(m.Tier)
			if !v.Anchored {
				m.MMR = 0
			}
			v.Marks = append(v.Marks, m)
		}
	}
	period := games[from:]
	v.Weekly = v.Days > 0 && v.Days <= 180
	heroes, lanes, parties, periods := map[string]*RankBucket{}, map[string]*RankBucket{}, map[string]*RankBucket{}, map[string]*RankBucket{}
	var periodKeys []string
	add := func(m map[string]*RankBucket, key string, win bool) *RankBucket {
		b := m[key]
		if b == nil {
			b = &RankBucket{Key: key}
			m[key] = b
		}
		b.add(win)
		return b
	}
	run := 0
	v.Peak, v.Low = math.Inf(-1), math.Inf(1)
	t7, t30 := now.AddDate(0, 0, -7).Unix(), now.AddDate(0, 0, -30).Unix()
	for i := range period {
		g := &period[i]
		mmr := offset + cum[from+i]
		v.Points = append(v.Points, RankPoint{MatchID: g.MatchID, EndTime: g.EndTime, Win: g.Win, HeroName: g.HeroName, OpenDota: g.OpenDota, MMR: mmr})
		if mmr > v.Peak {
			v.Peak, v.PeakAt = mmr, g.EndTime
		}
		v.Low = math.Min(v.Low, mmr)
		d := -MMRPerGame
		if g.Win {
			v.Wins++
			d = MMRPerGame
		}
		v.Games++
		v.Net += d
		if g.EndTime >= t7 {
			v.Net7 += d
		}
		if g.EndTime >= t30 {
			v.Net30 += d
		}
		if g.Win == (run > 0) && run != 0 {
			if run > 0 {
				run++
			} else {
				run--
			}
		} else if g.Win {
			run = 1
		} else {
			run = -1
		}
		v.BestRun = max(v.BestRun, run)
		v.WorstRun = max(v.WorstRun, -run)

		add(heroes, g.HeroName, g.Win)
		if r := laneRole(g); r != "" {
			add(lanes, r, g.Win)
		}
		switch {
		case g.PartySize == 1:
			add(parties, "solo", g.Win)
		case g.PartySize > 1:
			add(parties, "party", g.Win)
		}
		end := time.Unix(g.EndTime, 0)
		key := end.Format("2006-01")
		if v.Weekly {
			monday := end.AddDate(0, 0, -((int(end.Weekday()) + 6) % 7))
			key = monday.Format("2006-01-02")
		}
		if periods[key] == nil {
			periodKeys = append(periodKeys, key)
		}
		add(periods, key, g.Win)
	}
	v.Streak = run
	if v.Games == 0 {
		v.Peak, v.Low = 0, 0
	}
	list := func(m map[string]*RankBucket, minGames int) []RankBucket {
		out := []RankBucket{}
		for _, b := range m {
			if b.Games >= minGames {
				out = append(out, *b)
			}
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Net != out[j].Net {
				return out[i].Net > out[j].Net
			}
			return out[i].Games > out[j].Games
		})
		return out
	}
	v.ByHero = list(heroes, rankMinGames)
	v.ByLane = list(lanes, rankMinGames)
	v.ByParty = list(parties, 1)
	for _, k := range periodKeys {
		v.ByPeriod = append(v.ByPeriod, *periods[k])
	}
}
