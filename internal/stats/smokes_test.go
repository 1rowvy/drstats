package stats

import (
	"path/filepath"
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
	"github.com/1rowvy/drstats/internal/store"
)

func TestAnalyzeSmokes(t *testing.T) {
	teams := map[int]int{0: 2, 1: 2, 2: 2, 5: 3, 6: 3}
	smokes := []parser.Smoke{
		// Smoke 1: 0 smokes 0 and 1 at 600, found at 620.
		{T: 600, End: 620, PlayerID: 0, UserPID: 0},
		{T: 600.5, End: 625, PlayerID: 1, UserPID: 0},
		// Smoke 2: 5 smokes 5 and 6 at 1200; expires.
		{T: 1200, End: 1245, PlayerID: 5, UserPID: 5},
		{T: 1200, End: 1245, PlayerID: 6, UserPID: 5},
		// Smoke 3: 2 alone at 1500, a tower falls after it.
		{T: 1500, End: 1530, PlayerID: 2, UserPID: 2},
	}
	deaths := []parser.Death{
		{T: 630, VictimPID: 5, KillerPID: 1, Assists: []int{0}},  // smoke 1 kill, 0 assists
		{T: 700, VictimPID: 6, KillerPID: 0, Assists: []int{}},   // too late for smoke 1
		{T: 1250, VictimPID: 0, KillerPID: 5, Assists: []int{6}}, // enemy smoke 2 catches 0
		{T: 1255, VictimPID: 1, KillerPID: 6, Assists: []int{}},
	}
	objs := []parser.Objective{{T: 1560, Kind: "tower", Key: "npc_dota_badguys_tower2_top", Team: 3, KillerPID: 2}}
	got := analyzeSmokes(smokes, deaths, objs, teams, 0)
	if len(got) != 3 {
		t.Fatalf("smokes: %+v", got)
	}
	s1, s2, s3 := got[0], got[1], got[2]
	if !s1.Mine || s1.Enemy || len(s1.Members) != 2 || s1.Break != 620 || s1.End != 625 ||
		s1.Result != SmokeKill || !s1.Involved || s1.Died || len(s1.Kills) != 1 {
		t.Errorf("smoke 1: %+v", s1)
	}
	if s2.Mine || !s2.Enemy || !s2.Died || s2.Result != SmokeKill || len(s2.Kills) != 2 {
		t.Errorf("smoke 2: %+v", s2)
	}
	if s3.Mine || s3.Enemy || s3.Result != SmokeObjective || s3.Objective == "" {
		t.Errorf("smoke 3: %+v", s3)
	}

	games := []Game{{GameRow: store.GameRow{MatchID: 1}, Win: true}}
	st := smokeStats(games, map[int64]store.SmokeGame{1: {PlayerID: 0, Teams: teams, Smokes: smokes, Deaths: deaths, Objectives: objs}})
	if st.Games != 1 || st.Smokes != 1 || st.Used != 1 || st.Kill != 1 || st.Involved != 1 ||
		st.Enemy != 1 || st.Caught != 1 || st.CaughtGames != 1 || st.NGood != 1 {
		t.Errorf("stats: %+v", st)
	}
}

func TestSmokesOnReplay(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "*.dem"))
	if len(files) == 0 {
		t.Skip("no replay in testdata/, skipping")
	}
	m, err := parser.ParseFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, _, _ := st.AddReplay(files[0], 1, 1)
	mid, err := st.SaveMatch(id, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range m.Players {
		if p.AccountID == 0 {
			continue
		}
		v, err := BuildMatchView(st, mid, 0, p.PlayerID, DefaultOptions)
		if err != nil {
			t.Fatal(err)
		}
		mine := 0
		for _, s := range v.Smokes {
			if s.Mine && !s.Enemy {
				mine++
			}
			if s.Region == "" {
				t.Errorf("%s smoke at %.0f: no region", p.HeroName, s.T)
			}
		}
		games, err := loadGames(st, p.AccountID)
		if err != nil {
			t.Fatal(err)
		}
		sg, err := st.SmokeGames(p.AccountID)
		if err != nil {
			t.Fatal(err)
		}
		ss := smokeStats(games, sg)
		if ss.Games != 1 || ss.Smokes != mine {
			t.Errorf("%s: stats %+v, view has %d own smokes", p.HeroName, ss, mine)
		}
		if p.PlayerID == 0 {
			for _, s := range v.Smokes {
				t.Logf("%6.0f–%-6.0f team %d user %d members %v kills %v lost %v obj %q -> %s at %s",
					s.T, s.End, s.Team, s.User, s.Members, s.Kills, s.Lost, s.Objective, s.Result, s.Region)
			}
		}
	}
}
