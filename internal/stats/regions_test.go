package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/parser"
)

func TestRegion(t *testing.T) {
	cases := []struct {
		x, y float64
		want string
	}{
		{-6700, -6200, "base:radiant"},
		{6500, 6000, "base:dire"},
		{-700, -400, "river:mid"},
		{-6600, 2000, "lane:top:radiant"},
		{0, 6000, "lane:top:dire"},
		{3000, 3000, "lane:mid:dire"},
		{6200, -2000, "lane:bot:dire"},
		{1000, 4000, "jungle:dire"},
		{-1000, -4000, "jungle:radiant"},
	}
	for _, c := range cases {
		if got := Region(c.x, c.y); got != c.want {
			t.Errorf("Region(%v, %v) = %q, want %q", c.x, c.y, got, c.want)
		}
	}
}

func TestDeathContextVision(t *testing.T) {
	players := []parser.Player{{PlayerID: 0, Team: parser.TeamRadiant}, {PlayerID: 5, Team: parser.TeamDire}}
	deaths := []parser.Death{
		{T: 100, VictimPID: 0, X: 0, Y: 0, HasPos: true},
		{T: 300, VictimPID: 0, X: 0, Y: 0, HasPos: true}, // ward expired
		{T: 100, VictimPID: 5, X: 0, Y: 0, HasPos: true}, // enemy ward does not count
	}
	wards := []parser.Ward{{T: 50, End: 200, Team: parser.TeamRadiant, X: 500, Y: 500}}
	ctx := deathContext(players, deaths, wards, nil)
	if !ctx[0].Vision || ctx[1].Vision || ctx[2].Vision {
		t.Fatalf("vision = %v %v %v, want true false false", ctx[0].Vision, ctx[1].Vision, ctx[2].Vision)
	}
}
