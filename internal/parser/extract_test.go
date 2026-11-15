package parser

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// openDota is the subset of OpenDota's /matches/{id} response used to cross-check.
type openDota struct {
	MatchID  uint64 `json:"match_id"`
	Duration int    `json:"duration"`
	Players  []struct {
		AccountID uint32 `json:"account_id"`
		HeroID    int    `json:"hero_id"`
		Kills     int    `json:"kills"`
		Deaths    int    `json:"deaths"`
		Assists   int    `json:"assists"`
		LastHits  int    `json:"last_hits"`
		LhT       []int  `json:"lh_t"`
		GoldT     []int  `json:"gold_t"`
		XpT       []int  `json:"xp_t"`
		DeathsLog []any  `json:"deaths_log"`
	} `json:"players"`
}

func TestParseReplay(t *testing.T) {
	path := testReplay(t)
	m, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.MatchID == 0 || len(m.Players) != 10 || m.Winner == 0 || m.Duration < 600 || m.EndTime == 0 {
		t.Fatalf("bad meta: id=%d players=%d winner=%d dur=%v end=%d", m.MatchID, len(m.Players), m.Winner, m.Duration, m.EndTime)
	}
	for _, p := range m.Players {
		if !strings.HasPrefix(p.HeroName, "npc_dota_hero_") || p.HeroID == 0 {
			t.Errorf("player %d: hero %q / %d", p.PlayerID, p.HeroName, p.HeroID)
		}
	}
	if n := len(m.Series); n < 10*int(m.Duration/SeriesStep) {
		t.Errorf("series too short: %d samples", n)
	}
	var deaths int
	for _, p := range m.Players {
		deaths += p.Deaths
	}
	withPos := 0
	for _, d := range m.Deaths {
		if d.HasPos {
			withPos++
		}
	}
	t.Logf("match %d, %.0fs, %d samples, %d deaths (%d with position), scoreboard deaths %d",
		m.MatchID, m.Duration, len(m.Series), len(m.Deaths), withPos, deaths)
	if withPos < len(m.Deaths)*9/10 {
		t.Errorf("only %d of %d deaths have a position", withPos, len(m.Deaths))
	}

	od := loadOpenDota(t, path)
	if od == nil {
		return
	}
	if od.MatchID != m.MatchID {
		t.Fatalf("match id %d vs OpenDota %d", m.MatchID, od.MatchID)
	}
	for _, op := range od.Players {
		var p *Player
		for i := range m.Players {
			if m.Players[i].HeroID == op.HeroID {
				p = &m.Players[i]
			}
		}
		if p == nil {
			t.Errorf("hero %d missing", op.HeroID)
			continue
		}
		if p.Kills != op.Kills || p.Deaths != op.Deaths || p.Assists != op.Assists || p.LastHits != op.LastHits {
			t.Errorf("hero %d: KDA/LH %d/%d/%d %d, OpenDota %d/%d/%d %d", op.HeroID,
				p.Kills, p.Deaths, p.Assists, p.LastHits, op.Kills, op.Deaths, op.Assists, op.LastHits)
		}
		if op.AccountID != 0 && p.AccountID != op.AccountID {
			t.Errorf("hero %d: account %d vs %d", op.HeroID, p.AccountID, op.AccountID)
		}
		got := 0
		for _, d := range m.Deaths {
			if d.VictimPID == p.PlayerID {
				got++
			}
		}
		if got != len(op.DeathsLog) {
			t.Errorf("hero %d: %d deaths extracted, OpenDota deaths_log %d", op.HeroID, got, len(op.DeathsLog))
		}
		// Series at 10:00 must match OpenDota's per-minute arrays.
		for _, s := range m.Series {
			if s.PlayerID == p.PlayerID && s.T == 600 && len(op.LhT) > 10 {
				if s.LastHits != op.LhT[10] || s.TotalXP != op.XpT[10] {
					t.Errorf("hero %d @10: lh %d xp %d, OpenDota lh %d xp %d", op.HeroID, s.LastHits, s.TotalXP, op.LhT[10], op.XpT[10])
				}
			}
		}
	}
}

// loadOpenDota reads testdata/<match>.opendota.json next to the replay, if present.
func loadOpenDota(t *testing.T, replay string) *openDota {
	b, err := os.ReadFile(strings.TrimSuffix(replay, ".dem") + ".opendota.json")
	if err != nil {
		t.Log("no OpenDota json next to the replay, skipping cross-check")
		return nil
	}
	var od openDota
	if err := json.Unmarshal(b, &od); err != nil {
		t.Fatal(err)
	}
	return &od
}
