package parser

// Version of the extraction logic. Bump it whenever what Parse extracts (or how)
// changes, so stored matches can be detected as stale and re-parsed.
const Version = 2

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

	Purchases []Purchase   `json:"purchases"`
	Casts     []Cast       `json:"casts"`
	Hits      []Hit        `json:"hits"`
	Combat    []CombatStat `json:"combat"`
	Wards     []Ward       `json:"wards"`
	Runes     []Rune       `json:"runes"`
	Stacks    []Mark       `json:"stacks"`
	Buybacks  []Mark       `json:"buybacks"`
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

	// Earned gold by source (final, cumulative).
	GoldCreep    int `json:"gold_creep"`    // lane creep kills and denies
	GoldNeutral  int `json:"gold_neutral"`  // neutral creep kills
	GoldHero     int `json:"gold_hero"`     // hero kills and assists
	GoldBuilding int `json:"gold_building"` // towers, barracks, outposts
	GoldPassive  int `json:"gold_passive"`  // periodic income
	GoldBounty   int `json:"gold_bounty"`   // bounty runes
	GoldRoshan   int `json:"gold_roshan"`
	GoldOther    int `json:"gold_other"` // everything else (couriers, wards, abilities, comeback...)

	GoldLost       int     `json:"gold_lost"` // lost on death
	Buybacks       int     `json:"buybacks"`  // count
	CampsStacked   int     `json:"camps_stacked"`
	Stuns          float64 `json:"stuns"` // seconds of stun dealt
	ObsPlaced      int     `json:"obs_placed"`
	SenPlaced      int     `json:"sen_placed"`
	WardsDestroyed int     `json:"wards_destroyed"`
	RunePickups    int     `json:"rune_pickups"`
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
	// Cumulative farm gold and damage (all targets), for gold-source charts
	// and idle detection.
	GoldCreep   int `json:"gold_creep"`
	GoldNeutral int `json:"gold_neutral"`
	DmgOut      int `json:"dmg_out"` // damage dealt by the hero, its illusions and summons
	DmgIn       int `json:"dmg_in"`  // damage taken by the hero
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
	GoldLost  int     `json:"gold_lost"` // gold lost on this death
	Buyback   bool    `json:"buyback"`   // the victim bought back before respawning
}

// Purchase is an item bought by a player (item key without the "item_" prefix).
type Purchase struct {
	T        float64 `json:"t"`
	PlayerID int     `json:"pid"`
	Item     string  `json:"item"`
}

// Cast is a use of an ability or an active item by a hero.
type Cast struct {
	T        float64 `json:"t"`
	PlayerID int     `json:"pid"`
	Ability  string  `json:"ability"` // ability key or item_* key
}

// Hit is damage dealt or a debuff applied to an enemy (non-illusion) hero by
// a player's ability or item. Repeated hits of the same ability within
// HitMerge seconds are merged. Used for the hit-rate heuristic.
type Hit struct {
	T        float64 `json:"t"`
	PlayerID int     `json:"pid"`
	Ability  string  `json:"ability"`
}

// HitMerge is the merge window of Hit events, seconds.
const HitMerge = 0.2

// Combat stat kinds.
const (
	DmgOut  = "dmg_out"  // player's damage to Other
	DmgIn   = "dmg_in"   // damage received by the player from Other
	HealOut = "heal_out" // healing done by the player to Other
)

// CombatStat is an aggregate of combat log damage/heal entries for one player.
// Other is the hero npc name of the other side, or a category: creep, neutral,
// building, roshan, illusion, ward, unit.
type CombatStat struct {
	PlayerID  int    `json:"pid"`
	Kind      string `json:"kind"`
	Other     string `json:"other"`
	Inflictor string `json:"inflictor"` // ability/item key; "" = autoattack
	Amount    int    `json:"amount"`
	Count     int    `json:"count"`
}

// Ward is an observer or sentry ward placed by a player.
type Ward struct {
	T        float64 `json:"t"`
	End      float64 `json:"end"` // removal time (expired or destroyed)
	PlayerID int     `json:"pid"`
	Team     int     `json:"team"`
	Sentry   bool    `json:"sentry"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	// Destroyed by an enemy: KillerPID is the player (-1 when not a hero),
	// Killer the npc name of the attacker.
	Destroyed bool   `json:"destroyed"`
	KillerPID int    `json:"killer"`
	Killer    string `json:"killer_name"`
}

// Rune types (DOTA_RUNES).
const (
	RuneDoubleDamage = 0
	RuneHaste        = 1
	RuneIllusion     = 2
	RuneInvis        = 3
	RuneRegen        = 4
	RuneBounty       = 5
	RuneArcane       = 6
	RuneWater        = 7
	RuneWisdom       = 8
	RuneShield       = 9
)

// Rune is a rune picked up (or bottled) by a player.
type Rune struct {
	T        float64 `json:"t"`
	PlayerID int     `json:"pid"`
	Type     int     `json:"type"`
	Bottled  bool    `json:"bottled"`
}

// Mark is a timestamped per-player event without payload (stack, buyback).
type Mark struct {
	T        float64 `json:"t"`
	PlayerID int     `json:"pid"`
}

// AccountFromSteam64 converts a steam64 id to an account id (0 for bots).
func AccountFromSteam64(s uint64) uint32 {
	if s <= Steam64Base {
		return 0
	}
	return uint32(s - Steam64Base)
}
