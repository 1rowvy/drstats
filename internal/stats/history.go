package stats

import (
	"math"
	"sort"
	"strconv"

	"github.com/1rowvy/drstats/internal/opendota"
	"github.com/1rowvy/drstats/internal/store"
)

// Dashboard sources (Filter.Source).
const (
	SourceReplays = ""    // parsed replays only
	SourceAll     = "all" // replays + OpenDota history of the matches without a replay
)

// Team ids as stored for replays.
const (
	teamRadiant = 2
	teamDire    = 3
)

// historyLane maps OpenDota's lane (1 bot, 2 mid, 3 top, 4/5 jungle) to ours.
func historyLane(m opendota.HistoryMatch) string {
	if m.IsRoaming {
		return LaneRoam
	}
	switch m.Lane {
	case 1:
		return LaneBot
	case 2:
		return LaneMid
	case 3:
		return LaneTop
	case 4, 5:
		return LaneJungle
	}
	return ""
}

// historyGame converts an OpenDota history match into a Game. Values a
// history row does not have (laning, wards, stacks, gold lost, team kills)
// stay zero / nil and the metrics skip them for OpenDota rows.
func historyGame(m opendota.HistoryMatch, heroName func(int) string) Game {
	team, winner := teamDire, teamDire
	if m.Radiant() {
		team = teamRadiant
	}
	if m.RadiantWin {
		winner = teamRadiant
	}
	mins := float64(m.Duration) / 60
	r := store.GameRow{
		MatchID: m.MatchID, EndTime: m.StartTime + int64(m.Duration), Duration: float64(m.Duration), Winner: winner,
		GameMode: m.GameMode, LobbyType: m.LobbyType, Team: team, HeroID: m.HeroID, HeroName: heroName(m.HeroID),
		Kills: m.Kills, Deaths: m.Deaths, Assists: m.Assists, LastHits: m.LastHits, Denies: m.Denies,
		TotalGold: int(math.Round(float64(m.GoldPerMin) * mins)), TotalXP: int(math.Round(float64(m.XPPerMin) * mins)),
		HeroDamage: m.HeroDamage, TowerDamage: m.TowerDamage, Healing: m.HeroHealing, PartySize: m.PartySize,
	}
	return Game{GameRow: r, Win: m.Win(), Lane: historyLane(m), OpenDota: true}
}

// mergeHistory adds the history matches that have no replay to the replay
// games (which win: they carry far more data) and fills the party size of
// replay games OpenDota extras did not cover. Result is oldest first.
func mergeHistory(replays []Game, hist []opendota.HistoryMatch, heroName func(int) string, addMissing bool) []Game {
	byID := make(map[int64]opendota.HistoryMatch, len(hist))
	for _, m := range hist {
		byID[m.MatchID] = m
	}
	out := make([]Game, 0, len(replays)+len(hist))
	seen := make(map[int64]bool, len(replays))
	for _, g := range replays {
		if m, ok := byID[g.MatchID]; ok && g.PartySize == 0 {
			g.PartySize = m.PartySize
		}
		seen[g.MatchID] = true
		out = append(out, g)
	}
	if addMissing {
		for _, m := range hist {
			if !seen[m.MatchID] {
				out = append(out, historyGame(m, heroName))
			}
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].EndTime != out[j].EndTime {
				return out[i].EndTime < out[j].EndTime
			}
			return out[i].MatchID < out[j].MatchID
		})
	}
	return out
}

// HeroNamer returns a hero-id -> npc name lookup with a stable fallback.
func HeroNamer(byID map[int]string) func(int) string {
	return func(id int) string {
		if n := byID[id]; n != "" {
			return n
		}
		return "npc_dota_hero_" + strconv.Itoa(id)
	}
}
