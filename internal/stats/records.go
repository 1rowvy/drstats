package stats

import (
	"time"

	"github.com/1rowvy/drstats/internal/store"
)

// ActivityWeeks is how many weeks the activity calendar covers.
const ActivityWeeks = 12

// DayBucket counts the games of one local calendar day.
type DayBucket struct {
	Date  string `json:"date"` // YYYY-MM-DD, local time of the game start
	Games int    `json:"games"`
	Wins  int    `json:"wins"`
}

// Record is a best-ever value over the filtered matches.
type Record struct {
	Key      string  `json:"key"` // max_gpm, max_kills, max_hero_dmg, longest
	Value    float64 `json:"value"`
	MatchID  int64   `json:"match_id"`
	HeroName string  `json:"hero_name"`
	EndTime  int64   `json:"end_time"`
	Win      bool    `json:"win"`
	OpenDota bool    `json:"opendota"` // no replay
}

// Streak is the longest run of wins.
type Streak struct {
	Len  int   `json:"len"`
	From int64 `json:"from"` // end time of the first and last match of the run
	To   int64 `json:"to"`
}

// Extras are the dashboard-only summaries.
type Extras struct {
	Hours   float64     `json:"hours"`
	Days    []DayBucket `json:"days"` // last ActivityWeeks weeks, only days with games
	Records []Record    `json:"records"`
	Streak  Streak      `json:"streak"`
	// Party / solo by OpenDota party size (matches without it are skipped).
	Party   Bucket         `json:"party"`
	Solo    Bucket         `json:"solo"`
	Fastest []FastItemView `json:"fastest"` // earliest first purchase per item
}

// FastItemView is store.FastItem with the match context.
type FastItemView struct {
	store.FastItem
	HeroName string `json:"hero_name"`
	EndTime  int64  `json:"end_time"`
}

func buildExtras(games []Game, now time.Time) Extras {
	x := Extras{Days: []DayBucket{}, Records: []Record{}, Fastest: []FastItemView{}, Party: Bucket{Key: "party"}, Solo: Bucket{Key: "solo"}}
	cutoff := now.AddDate(0, 0, -7*ActivityWeeks)
	days := map[string]*DayBucket{}
	var order []string
	best := map[string]*Record{}
	consider := func(key string, v float64, g *Game) {
		r := best[key]
		if r == nil || v > r.Value {
			best[key] = &Record{Key: key, Value: v, MatchID: g.MatchID, HeroName: g.HeroName, EndTime: g.EndTime, Win: g.Win, OpenDota: g.OpenDota}
		}
	}
	run := Streak{}
	for i := range games {
		g := &games[i]
		x.Hours += g.Duration / 3600
		start := gameStart(g)
		if start.After(cutoff) {
			k := start.Format("2006-01-02")
			d := days[k]
			if d == nil {
				d = &DayBucket{Date: k}
				days[k] = d
				order = append(order, k)
			}
			d.Games++
			if g.Win {
				d.Wins++
			}
		}
		if gpm, ok := perMin(g.TotalGold, g); ok {
			consider("max_gpm", gpm, g)
		}
		consider("max_kills", float64(g.Kills), g)
		consider("max_hero_dmg", float64(g.HeroDamage), g)
		consider("longest", g.Duration, g)
		if g.Win {
			if run.Len == 0 {
				run.From = g.EndTime
			}
			run.Len++
			run.To = g.EndTime
			if run.Len > x.Streak.Len {
				x.Streak = run
			}
		} else {
			run = Streak{}
		}
		switch {
		case g.PartySize > 1:
			addBucket(&x.Party, g)
		case g.PartySize == 1:
			addBucket(&x.Solo, g)
		}
	}
	finishBucket(&x.Party)
	finishBucket(&x.Solo)
	for _, k := range order {
		x.Days = append(x.Days, *days[k])
	}
	for _, k := range []string{"max_gpm", "max_kills", "max_hero_dmg", "longest"} {
		if r := best[k]; r != nil {
			x.Records = append(x.Records, *r)
		}
	}
	return x
}
