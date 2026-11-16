package stats

import "strconv"

// SessionGap is the pause between two games that starts a new session.
const SessionGap = 3600 // seconds

// SessionMaxN is the last game-in-session bucket ("4+").
const SessionMaxN = 4

// CurStreak is the run of equal results ending at the latest match.
type CurStreak struct {
	Len int  `json:"len"`
	Win bool `json:"win"`
}

// Tilt summarises how results depend on the previous game and on the
// position of a game inside a session (games with gaps < SessionGap).
type Tilt struct {
	// Session buckets: key "1".."4" — 1st, 2nd, 3rd, 4th-and-later game of a session.
	Session []Bucket `json:"session"`
	// Games right after a win / a loss inside the same session.
	AfterWin  Bucket    `json:"after_win"`
	AfterLoss Bucket    `json:"after_loss"`
	Current   CurStreak `json:"current"`
	// Sessions is how many sessions the games split into.
	Sessions int `json:"sessions"`
}

// buildTilt expects games oldest first.
func buildTilt(games []Game) Tilt {
	t := Tilt{Session: make([]Bucket, SessionMaxN), AfterWin: Bucket{Key: "after_win"}, AfterLoss: Bucket{Key: "after_loss"}}
	for i := range t.Session {
		t.Session[i].Key = strconv.Itoa(i + 1)
	}
	n := 0
	for i := range games {
		g := &games[i]
		start := g.EndTime - int64(g.Duration)
		if i == 0 || start-games[i-1].EndTime > SessionGap {
			n = 0
			t.Sessions++
		} else if games[i-1].Win {
			addBucket(&t.AfterWin, g)
		} else {
			addBucket(&t.AfterLoss, g)
		}
		n++
		addBucket(&t.Session[min(n, SessionMaxN)-1], g)
		if t.Current.Len > 0 && t.Current.Win == g.Win {
			t.Current.Len++
		} else {
			t.Current = CurStreak{Len: 1, Win: g.Win}
		}
	}
	for i := range t.Session {
		finishBucket(&t.Session[i])
	}
	finishBucket(&t.AfterWin)
	finishBucket(&t.AfterLoss)
	return t
}
