package parser

// Version of the extraction logic. Bump it whenever what Parse extracts (or how)
// changes, so stored matches can be detected as stale and re-parsed.
const Version = 1

// SeriesStep is the sampling interval of the per-player time series, game seconds.
const SeriesStep = 5.0

// Steam64Base converts between steam64 and the 32-bit account id.
const Steam64Base = 76561197960265728

// Teams.
const (
	TeamRadiant = 2
	TeamDire    = 3
)

// Match is everything extracted from one replay.
type Match struct {
	MatchID   uint64   `json:"match_id"`
	GameBuild uint32   `json:"game_build"`
	GameMode  int      `json:"game_mode"`
	LobbyType int      `json:"lobby_type"`
	Winner    int      `json:"winner"`   // TeamRadiant / TeamDire, 0 if unknown
	Duration  float64  `json:"duration"` // game seconds from the horn to POST_GAME
	EndTime   int64    `json:"end_time"` // unix seconds
	StartTick uint32   `json:"start_tick"`
	Players   []Player `json:"players"`
	Series    []Sample `json:"series"`
	Deaths    []Death  `json:"deaths"`
}

// Player is a final scoreboard line.
type Player struct {
	PlayerID    int    `json:"player_id"` // 0..9 (PlayerResource index)
	Team        int    `json:"team"`
	TeamSlot    int    `json:"team_slot"`
	HeroID      int    `json:"hero_id"`
	HeroName    string `json:"hero_name"` // npc_dota_hero_*
	Name        string `json:"name"`
	AccountID   uint32 `json:"account_id"`
	Kills       int    `json:"kills"`
	Deaths      int    `json:"deaths"`
	Assists     int    `json:"assists"`
	Level       int    `json:"level"`
	LastHits    int    `json:"last_hits"`
	Denies      int    `json:"denies"`
	NetWorth    int    `json:"net_worth"`
	TotalGold   int    `json:"total_gold"` // total earned
	TotalXP     int    `json:"total_xp"`
	HeroDamage  int    `json:"hero_damage"`
	TowerDamage int    `json:"tower_damage"`
	Healing     int    `json:"healing"`
	RankTier    int    `json:"rank_tier"`
}

// Sample is one point of a player's time series (cumulative counters).
type Sample struct {
	T         float64 `json:"t"`
	PlayerID  int     `json:"pid"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Alive     bool    `json:"alive"`
	Level     int     `json:"level"`
	Gold      int     `json:"gold"` // current (reliable + unreliable)
	NetWorth  int     `json:"nw"`
	TotalGold int     `json:"total_gold"`
	TotalXP   int     `json:"total_xp"`
	LastHits  int     `json:"lh"`
	Denies    int     `json:"dn"`
}

// Death is a (non-illusion) hero death.
type Death struct {
	T         float64 `json:"t"`
	VictimPID int     `json:"victim"`
	KillerPID int     `json:"killer"`      // -1 when the killing blow was not from a hero
	Killer    string  `json:"killer_name"` // npc name of the attacker (hero, creep, tower, ...)
	Inflictor string  `json:"inflictor"`   // ability/item of the killing blow; "" = autoattack
	Assists   []int   `json:"assists"`     // player ids
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	HasPos    bool    `json:"has_pos"`
}

// AccountFromSteam64 converts a steam64 id to an account id (0 for bots).
func AccountFromSteam64(s uint64) uint32 {
	if s <= Steam64Base {
		return 0
	}
	return uint32(s - Steam64Base)
}
