package stats

import (
	"path/filepath"
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

func TestLaneOpponent(t *testing.T) {
	players := []parser.Player{{PlayerID: 0, Team: 2}, {PlayerID: 1, Team: 2}, {PlayerID: 5, Team: 3}, {PlayerID: 6, Team: 3}}
	by := map[int][]parser.Sample{}
	for ti := 0; ti <= 120; ti++ {
		tm := float64(ti * 5)
		by[0] = append(by[0], parser.Sample{T: tm, PlayerID: 0, X: -6000, Y: 5000, Alive: true}) // top
		by[1] = append(by[1], parser.Sample{T: tm, PlayerID: 1, X: 0, Y: 0, Alive: true})        // mid
		by[5] = append(by[5], parser.Sample{T: tm, PlayerID: 5, X: 100, Y: 100, Alive: true})    // mid
		by[6] = append(by[6], parser.Sample{T: tm, PlayerID: 6, X: -5500, Y: 5800, Alive: true}) // top
	}
	if got := LaneOpponent(players, by, 0); got != 6 {
		t.Errorf("opponent of top = %d, want 6", got)
	}
	if got := LaneOpponent(players, by, 1); got != 5 {
		t.Errorf("opponent of mid = %d, want 5", got)
	}
}

func TestBuildMatchView(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, _, _ := st.AddReplay("/x.dem", 1, 1)
	m := &parser.Match{MatchID: 7, Duration: 600, Winner: 2, Players: []parser.Player{
		{PlayerID: 0, Team: 2, AccountID: 11, TotalGold: 3000, TotalXP: 6000},
		{PlayerID: 1, Team: 2, AccountID: 12},
		{PlayerID: 5, Team: 3, AccountID: 13},
	}}
	for ti := 0; ti <= 120; ti++ {
		tm := float64(ti * 5)
		for _, pid := range []int{0, 1, 5} {
			m.Series = append(m.Series, parser.Sample{T: tm, PlayerID: pid, Alive: true, TotalGold: ti * 25 * (pid + 1), NetWorth: 600 + ti})
		}
	}
	if _, err := st.SaveMatch(id, m); err != nil {
		t.Fatal(err)
	}
	v, err := BuildMatchView(st, 7, 11, -1, DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	if v.Focus != 0 || !v.IsMe || v.Opponent != 5 || v.Players[0].GPM != 300 || v.Players[0].XPM != 600 {
		t.Fatalf("view: focus %d me %v opp %d gpm %d", v.Focus, v.IsMe, v.Opponent, v.Players[0].GPM)
	}
	f := v.Farm
	if len(f.T) != 121 || f.GPM.Focus[0] != nil || *f.GPM.Focus[120] != 300 || *f.GPM.Opponent[120] != 1800 || *f.GPM.TeamAvg[120] != 600 {
		t.Errorf("farm gpm: %v %v %v", f.GPM.Focus[120], f.GPM.Opponent[120], f.GPM.TeamAvg[120])
	}
	// Picking another player explicitly.
	v, _ = BuildMatchView(st, 7, 11, 5, DefaultOptions)
	if v.Focus != 5 || v.IsMe {
		t.Errorf("explicit focus: %d %v", v.Focus, v.IsMe)
	}
	// Unknown account and no pick: no focus, empty lines.
	v, _ = BuildMatchView(st, 7, 0, -1, DefaultOptions)
	if v.Focus != -1 || len(v.Farm.NetWorth.Focus) != 0 {
		t.Errorf("no focus: %+v", v.Focus)
	}
}
