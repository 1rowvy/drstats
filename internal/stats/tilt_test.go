package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/store"
)

func TestBuildTilt(t *testing.T) {
	// Two sessions: W L L (back to back), then after a long pause L W.
	at := func(end int64, win bool) Game {
		return Game{GameRow: store.GameRow{EndTime: end, Duration: 1800}, Win: win}
	}
	gs := []Game{
		at(10000, true), at(12000, false), at(14000, false),
		at(100000, false), at(102000, true),
	}
	x := buildTilt(gs)
	if x.Sessions != 2 {
		t.Errorf("sessions %d", x.Sessions)
	}
	if x.Session[0].Games != 2 || x.Session[0].Wins != 1 || x.Session[1].Games != 2 || x.Session[2].Games != 1 {
		t.Errorf("session buckets %+v", x.Session)
	}
	if x.AfterWin.Games != 1 || x.AfterWin.Wins != 0 || x.AfterLoss.Games != 2 || x.AfterLoss.Wins != 1 {
		t.Errorf("after win %+v after loss %+v", x.AfterWin, x.AfterLoss)
	}
	if x.Current != (CurStreak{Len: 1, Win: true}) {
		t.Errorf("current %+v", x.Current)
	}
}
