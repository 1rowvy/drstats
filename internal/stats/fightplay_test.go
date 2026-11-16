package stats

import (
	"testing"

	"github.com/1rowvy/drstats/internal/constants"
	"github.com/1rowvy/drstats/internal/parser"
)

func TestFightPlay(t *testing.T) {
	R, D := parser.TeamRadiant, parser.TeamDire
	players := []parser.Player{{PlayerID: 0, Team: R, HeroName: "npc_dota_hero_necrolyte"}, {PlayerID: 5, Team: D, HeroName: "npc_dota_hero_lion"}}
	cds := map[string]constants.Cooldown{
		"necrolyte_reapers_scythe": {Max: 100, Levels: 3},
		"juggernaut_omni_slash":    {Max: 120, Levels: 3},
		"item_black_king_bar":      {Max: 95, Item: true},
	}
	m := &parser.Match{
		Hits: []parser.Hit{{T: 601, PlayerID: 5, Ability: "lion_impale"}, {T: 614, PlayerID: 0, Ability: "necrolyte_death_pulse"}},
		HeroDamage: []parser.DamageBucket{
			{T: 600, PlayerID: 5, Target: 0, Amount: 400},
			{T: 610, PlayerID: 0, Target: 5, Amount: 300},
		},
		Casts: []parser.Cast{
			{T: 300, PlayerID: 0, Ability: "necrolyte_reapers_scythe"},
			{T: 500, PlayerID: 0, Ability: "juggernaut_omni_slash"}, // stolen-like: not his
			{T: 603, PlayerID: 0, Ability: "item_black_king_bar"},
		},
		Purchases: []parser.Purchase{{T: 550, PlayerID: 0, Item: "black_king_bar"}},
		Deaths:    []parser.Death{{T: 618, VictimPID: 0, KillerPID: 5}},
	}
	byPID := map[int][]parser.Sample{0: {{T: 600, X: -3000, Y: 0, Alive: true, Level: 12}}}
	b := &Battle{X: 0, Y: 0, Deaths: []int{0}}
	fp := fightPlay(m, b, 600, 620, players, byPID, 0, cds)
	if fp.Initiator != 5 || fp.WeStarted || fp.InitAbility != "lion_impale" || fp.FirstTarget != 0 {
		t.Fatalf("opening %+v", fp)
	}
	if fp.MyEntry == nil || *fp.MyEntry != 9 || fp.Late || fp.MyDist != 3000 {
		t.Fatalf("entry %+v (%v)", fp, fp.MyEntry)
	}
	if len(fp.KeyUsed) != 1 || fp.KeyUsed[0] != "item_black_king_bar" || len(fp.KeyUnused) != 1 || fp.KeyUnused[0] != "necrolyte_reapers_scythe" {
		t.Fatalf("keys used %v unused %v", fp.KeyUsed, fp.KeyUnused)
	}
	if fp.DeathAfter == nil || *fp.DeathAfter != 17 {
		t.Fatalf("death after %v", fp.DeathAfter)
	}
}
