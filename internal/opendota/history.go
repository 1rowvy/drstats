package opendota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// PlayersURL is the OpenDota players endpoint (replaceable in tests).
var PlayersURL = "https://api.opendota.com/api/players/"

// HistoryMatch is one match of a player's OpenDota history: only the basic
// fields OpenDota has for every match (no replay parse needed).
type HistoryMatch struct {
	MatchID      int64 `json:"match_id"`
	StartTime    int64 `json:"start_time"`
	Duration     int   `json:"duration"` // seconds
	GameMode     int   `json:"game_mode"`
	LobbyType    int   `json:"lobby_type"`
	RadiantWin   bool  `json:"radiant_win"`
	PlayerSlot   int   `json:"player_slot"` // < 128 = Radiant
	HeroID       int   `json:"hero_id"`
	Kills        int   `json:"kills"`
	Deaths       int   `json:"deaths"`
	Assists      int   `json:"assists"`
	LastHits     int   `json:"last_hits"`
	Denies       int   `json:"denies"`
	GoldPerMin   int   `json:"gold_per_min"`
	XPPerMin     int   `json:"xp_per_min"`
	HeroDamage   int   `json:"hero_damage"`
	TowerDamage  int   `json:"tower_damage"`
	HeroHealing  int   `json:"hero_healing"`
	Lane         int   `json:"lane"` // 1 bot, 2 mid, 3 top, 4/5 jungle; 0 unknown
	LaneRole     int   `json:"lane_role"`
	IsRoaming    bool  `json:"is_roaming"`
	PartySize    int   `json:"party_size"`
	LeaverStatus int   `json:"leaver_status"`
}

// Radiant reports whether the player was on Radiant.
func (h HistoryMatch) Radiant() bool { return h.PlayerSlot < 128 }

// Win reports whether the player won.
func (h HistoryMatch) Win() bool { return h.Radiant() == h.RadiantWin }

var historyFields = []string{
	"match_id", "start_time", "duration", "game_mode", "lobby_type", "radiant_win", "player_slot", "hero_id",
	"kills", "deaths", "assists", "last_hits", "denies", "gold_per_min", "xp_per_min",
	"hero_damage", "tower_damage", "hero_healing", "lane", "lane_role", "is_roaming", "party_size", "leaver_status",
}

// historyMatchJSON mirrors HistoryMatch with nullable fields (OpenDota sends
// null for values it does not know, e.g. damage of very old matches).
type historyMatchJSON struct {
	MatchID      int64 `json:"match_id"`
	StartTime    int64 `json:"start_time"`
	Duration     *int  `json:"duration"`
	GameMode     *int  `json:"game_mode"`
	LobbyType    *int  `json:"lobby_type"`
	RadiantWin   *bool `json:"radiant_win"`
	PlayerSlot   *int  `json:"player_slot"`
	HeroID       int   `json:"hero_id"`
	Kills        *int  `json:"kills"`
	Deaths       *int  `json:"deaths"`
	Assists      *int  `json:"assists"`
	LastHits     *int  `json:"last_hits"`
	Denies       *int  `json:"denies"`
	GoldPerMin   *int  `json:"gold_per_min"`
	XPPerMin     *int  `json:"xp_per_min"`
	HeroDamage   *int  `json:"hero_damage"`
	TowerDamage  *int  `json:"tower_damage"`
	HeroHealing  *int  `json:"hero_healing"`
	Lane         *int  `json:"lane"`
	LaneRole     *int  `json:"lane_role"`
	IsRoaming    *bool `json:"is_roaming"`
	PartySize    *int  `json:"party_size"`
	LeaverStatus *int  `json:"leaver_status"`
}

// DecodeHistory converts an OpenDota /players/{id}/matches response.
// Matches without an id or a hero are dropped.
func DecodeHistory(body []byte) ([]HistoryMatch, error) {
	var raw []historyMatchJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode OpenDota history: %w", err)
	}
	i := func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	}
	b := func(p *bool) bool { return p != nil && *p }
	out := make([]HistoryMatch, 0, len(raw))
	for _, r := range raw {
		if r.MatchID <= 0 || r.HeroID <= 0 || r.RadiantWin == nil || r.PlayerSlot == nil {
			continue
		}
		out = append(out, HistoryMatch{
			MatchID: r.MatchID, StartTime: r.StartTime, Duration: i(r.Duration), GameMode: i(r.GameMode),
			LobbyType: i(r.LobbyType), RadiantWin: b(r.RadiantWin), PlayerSlot: i(r.PlayerSlot), HeroID: r.HeroID,
			Kills: i(r.Kills), Deaths: i(r.Deaths), Assists: i(r.Assists), LastHits: i(r.LastHits), Denies: i(r.Denies),
			GoldPerMin: i(r.GoldPerMin), XPPerMin: i(r.XPPerMin), HeroDamage: i(r.HeroDamage), TowerDamage: i(r.TowerDamage),
			HeroHealing: i(r.HeroHealing), Lane: i(r.Lane), LaneRole: i(r.LaneRole), IsRoaming: b(r.IsRoaming),
			PartySize: i(r.PartySize), LeaverStatus: i(r.LeaverStatus),
		})
	}
	return out, nil
}

// FetchHistory loads the account's matches of every game mode. days > 0
// limits it to matches of the last N days (incremental sync); 0 = all.
func (c *Client) FetchHistory(ctx context.Context, accountID uint32, days int) ([]HistoryMatch, error) {
	q := url.Values{"significant": {"0"}}
	for _, f := range historyFields {
		q.Add("project", f)
	}
	if days > 0 {
		q.Set("date", strconv.Itoa(days))
	}
	u := PlayersURL + strconv.FormatUint(uint64(accountID), 10) + "/matches?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "drstats")
	// The full history of an active player is a few MB: allow more time.
	hc := *c.HTTP
	hc.Timeout = 0
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenDota: HTTP %d", resp.StatusCode)
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, err
	}
	return DecodeHistory(buf)
}
