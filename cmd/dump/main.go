// Command dump is a research spike: it parses a Dota 2 replay with manta and
// prints a JSON overview of the raw data the replay actually contains, so the
// domain model can be designed from facts rather than assumptions.
//
// Usage: go run ./cmd/dump [-samples N] [-full] [-at SECONDS] path/to/replay.dem > dump.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dotabuff/manta"
	"github.com/dotabuff/manta/dota"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/1rowvy/drstats/internal/parser"
)

// Classes whose full field list (with sample values) goes into the dump.
var interesting = regexp.MustCompile(`^(CDOTA_Unit_Hero_|CDOTA_PlayerResource$|CDOTA_DataRadiant$|CDOTA_DataDire$|CDOTAGamerulesProxy$|CDOTAPlayerController$|CDOTA_NPC_Observer_Ward|CDOTA_Unit_Courier$|CDOTA_Item_Rune$|CDOTA_BaseNPC_Tower$|CDOTA_Unit_Roshan$|CDOTATeam$|CDOTA_BaseNPC_Creep_Lane$|CDOTA_BaseNPC_Creep_Neutral$|CDOTA_NeutralSpawner$)`)

// steam64 = account id + this base.
const steam64Base = 76561197960265728

type output struct {
	File          string                    `json:"file"`
	ParseSeconds  float64                   `json:"parse_seconds"`
	GameBuild     uint32                    `json:"game_build"`
	LastTick      uint32                    `json:"last_tick"`
	Error         string                    `json:"error,omitempty"`
	FileInfo      json.RawMessage           `json:"file_info,omitempty"`
	Match         matchInfo                 `json:"match"`
	Players       []playerInfo              `json:"players"`
	States        []stateChange             `json:"game_rules_states"`
	Pauses        [][2]uint32               `json:"pauses"`
	CombatLog     map[string]*combatLogType `json:"combat_log"`
	TopModifiers  map[string]int            `json:"top_modifiers_added"`
	GameEvents    map[string]int            `json:"game_events"`
	UserMessages  map[string]int            `json:"user_messages"`
	EntityClasses map[string]*classInfo     `json:"entity_classes"`
	Timeline      []snapshot                `json:"timeline"`
	Wards         []wardInfo                `json:"wards"`
}

type matchInfo struct {
	MatchID        uint64  `json:"match_id"`
	GameMode       int     `json:"game_mode"`
	LobbyType      int     `json:"lobby_type"`
	Winner         int     `json:"winner_team"` // 2 radiant, 3 dire
	StartTick      uint32  `json:"start_tick"`
	GameStartTime  float64 `json:"game_start_server_time"`
	EndGameTime    float64 `json:"end_game_time"` // game time at POST_GAME
	EndUnix        uint32  `json:"end_unix"`
	TotalPausedTks int     `json:"total_paused_ticks"`
}

type playerInfo struct {
	PlayerID  int    `json:"player_id"`
	Name      string `json:"name"`
	Steam64   uint64 `json:"steam64"`
	AccountID uint32 `json:"account_id"`
	Team      int    `json:"team"`
	TeamSlot  int    `json:"team_slot"`
	HeroID    int    `json:"hero_id"`
	HeroClass string `json:"hero_class"`
	HeroName  string `json:"hero_name"` // from file info, npc_dota_hero_*
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Assists   int    `json:"assists"`
	RankTier  int    `json:"rank_tier"`
}

type combatLogType struct {
	Count   int              `json:"count"`
	Samples []map[string]any `json:"samples"`
}

type classInfo struct {
	Count  int            `json:"count"`
	Fields map[string]any `json:"fields,omitempty"`
}

type stateChange struct {
	Tick     uint32  `json:"tick"`
	GameTime float64 `json:"game_time"`
	State    int     `json:"state"`
	Name     string  `json:"name"`
}

type snapshot struct {
	GameTime float64 `json:"t"`
	PlayerID int     `json:"pid"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Level    int     `json:"lvl"`
	HP       int     `json:"hp"`
	Alive    bool    `json:"alive"`
	NetWorth int     `json:"nw"`
	Gold     int     `json:"gold"`
	XP       int     `json:"xp"`
	LH       int     `json:"lh"`
	DN       int     `json:"dn"`
	// gold breakdown, cumulative
	CreepGold   int `json:"g_creep"`
	NeutralGold int `json:"g_neutral"`
	HeroGold    int `json:"g_hero"`
	BuildGold   int `json:"g_building"`
	IncomeGold  int `json:"g_income"`
	BountyGold  int `json:"g_bounty"`
	OtherGold   int `json:"g_other"`
	TotalEarned int `json:"g_total"`
}

type wardInfo struct {
	Class    string  `json:"class"`
	Created  float64 `json:"created"`
	Removed  float64 `json:"removed,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Team     int     `json:"team"`
	OwnerPID int     `json:"owner_pid"` // player id (m_nPlayerOwnerID is 2*pid)
}

var stateNames = map[int]string{
	0: "INIT", 1: "WAIT_FOR_PLAYERS_TO_LOAD", 2: "HERO_SELECTION", 3: "STRATEGY_TIME",
	4: "PRE_GAME", 5: "GAME_IN_PROGRESS", 6: "POST_GAME", 7: "DISCONNECT",
	8: "TEAM_SHOWCASE", 9: "CUSTOM_GAME_SETUP", 10: "WAIT_FOR_MAP_TO_LOAD",
	11: "SCENARIO_SETUP", 12: "PLAYER_DRAFT",
}

func main() {
	samples := flag.Int("samples", 3, "combat log samples per type")
	full := flag.Bool("full", false, "dump fields of every entity class, not only interesting ones")
	at := flag.Float64("at", 600, "game time (s) at which entity field values are sampled")
	step := flag.Float64("step", 60, "timeline snapshot step, game seconds")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: dump [-samples N] [-full] [-at SEC] [-step SEC] replay.dem")
		os.Exit(2)
	}
	d := newDumper(flag.Arg(0), *samples, *full, *at, *step)
	err := parser.Safe(d.run)
	if err != nil {
		d.out.Error = err.Error()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(d.out)
	if err != nil {
		os.Exit(1)
	}
}

type rawEvent struct {
	tick uint32
	m    map[string]any
}

type dumper struct {
	path     string
	nSamples int
	full     bool
	at, step float64

	out   *output
	p     *manta.Parser
	clock parser.Clock

	lastState  int
	eventNames map[int32]string
	rawCL      map[string][]rawEvent // samples keep ticks; game time filled at the end
	wardsByIdx map[int32]int
	nextSnap   float64
	fileHeroes map[uint64]string // steam64 -> npc_dota_hero_*
}

func newDumper(path string, n int, full bool, at, step float64) *dumper {
	return &dumper{
		path: path, nSamples: n, full: full, at: at, step: step,
		lastState:  -1,
		eventNames: map[int32]string{},
		rawCL:      map[string][]rawEvent{},
		wardsByIdx: map[int32]int{},
		fileHeroes: map[uint64]string{},
		out: &output{
			File:          path,
			CombatLog:     map[string]*combatLogType{},
			TopModifiers:  map[string]int{},
			GameEvents:    map[string]int{},
			UserMessages:  map[string]int{},
			EntityClasses: map[string]*classInfo{},
		},
	}
}

func (d *dumper) run() error {
	f, err := os.Open(d.path)
	if err != nil {
		return err
	}
	defer f.Close()
	p, err := manta.NewStreamParser(f)
	if err != nil {
		return err
	}
	d.p = p
	d.register()

	started := time.Now()
	if err := p.Start(); err != nil {
		return err
	}
	d.out.ParseSeconds = time.Since(started).Seconds()
	d.out.GameBuild = p.GameBuild
	d.out.LastTick = p.Tick
	d.finish()
	return nil
}

// gt converts a tick to game time using what the clock knows so far.
// Valid for ticks after the horn even mid-parse (pauses up to now are known).
func (d *dumper) gt(tick uint32) float64 { return round2(d.clock.GameTime(tick)) }

func (d *dumper) name(idx uint32) string {
	s, _ := d.p.LookupStringByIndex("CombatLogNames", int32(idx))
	return s
}

func (d *dumper) register() {
	p := d.p

	p.Callbacks.OnCDemoFileInfo(func(m *dota.CDemoFileInfo) error {
		b, _ := protojson.Marshal(m)
		d.out.FileInfo = b
		gi := m.GetGameInfo().GetDota()
		d.out.Match.EndUnix = gi.GetEndTime()
		d.out.Match.MatchID = gi.GetMatchId()
		for _, pi := range gi.GetPlayerInfo() {
			d.fileHeroes[pi.GetSteamid()] = pi.GetHeroName()
		}
		return nil
	})

	p.Callbacks.OnCMsgSource1LegacyGameEventList(func(m *dota.CMsgSource1LegacyGameEventList) error {
		for _, e := range m.GetDescriptors() {
			d.eventNames[e.GetEventid()] = e.GetName()
		}
		return nil
	})
	p.Callbacks.OnCMsgSource1LegacyGameEvent(func(m *dota.CMsgSource1LegacyGameEvent) error {
		n := d.eventNames[m.GetEventid()]
		if n == "" {
			n = fmt.Sprintf("id_%d", m.GetEventid())
		}
		d.out.GameEvents[n]++
		return nil
	})

	p.Callbacks.OnCMsgDOTACombatLogEntry(func(m *dota.CMsgDOTACombatLogEntry) error {
		t := strings.TrimPrefix(m.GetType().String(), "DOTA_COMBATLOG_")
		c := d.out.CombatLog[t]
		if c == nil {
			c = &combatLogType{}
			d.out.CombatLog[t] = c
		}
		c.Count++
		if m.GetType() == dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_MODIFIER_ADD {
			d.out.TopModifiers[d.name(m.GetInflictorName())]++
		}
		if len(d.rawCL[t]) < d.nSamples {
			s := protoMap(m)
			s["_attacker"] = d.name(m.GetAttackerName())
			s["_target"] = d.name(m.GetTargetName())
			s["_damage_source"] = d.name(m.GetDamageSourceName())
			s["_inflictor"] = d.name(m.GetInflictorName())
			s["_target_source"] = d.name(m.GetTargetSourceName())
			if m.GetType() == dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_PURCHASE {
				s["_value_as_name"] = d.name(m.GetValue())
			}
			d.rawCL[t] = append(d.rawCL[t], rawEvent{p.Tick, s})
		}
		return nil
	})

	p.Callbacks.OnCDOTAUserMsg_ChatEvent(func(m *dota.CDOTAUserMsg_ChatEvent) error {
		d.out.UserMessages["ChatEvent:"+strings.TrimPrefix(m.GetType().String(), "CHAT_MESSAGE_")]++
		return nil
	})
	p.Callbacks.OnCDOTAUserMsg_ChatMessage(func(*dota.CDOTAUserMsg_ChatMessage) error {
		d.out.UserMessages["ChatMessage"]++
		return nil
	})
	p.Callbacks.OnCDOTAUserMsg_SpectatorPlayerUnitOrders(func(*dota.CDOTAUserMsg_SpectatorPlayerUnitOrders) error {
		d.out.UserMessages["SpectatorPlayerUnitOrders"]++
		return nil
	})
	p.Callbacks.OnCDOTAUserMsg_LocationPing(func(*dota.CDOTAUserMsg_LocationPing) error {
		d.out.UserMessages["LocationPing"]++
		return nil
	})

	p.OnEntity(d.onEntity)
	p.Callbacks.OnCNETMsg_Tick(func(*dota.CNETMsg_Tick) error { d.onTick(); return nil })
}

func (d *dumper) onEntity(e *manta.Entity, op manta.EntityOp) error {
	cls := e.GetClassName()
	ci := d.out.EntityClasses[cls]
	if ci == nil {
		ci = &classInfo{}
		d.out.EntityClasses[cls] = ci
	}
	if op.Flag(manta.EntityOpCreated) {
		ci.Count++
	}

	switch {
	case cls == "CDOTAGamerulesProxy":
		st := parser.Int(e, "m_pGameRules.m_nGameState")
		paused := parser.Int(e, "m_pGameRules.m_bGamePaused") == 1
		d.clock.SetPaused(d.p.Tick, paused)
		if st != d.lastState {
			d.lastState = st
			if st == parser.StateGameInProgress {
				d.clock.MarkStart(d.p.Tick)
			}
			d.out.States = append(d.out.States, stateChange{Tick: d.p.Tick, State: st, Name: stateNames[st]})
		}
		m := &d.out.Match
		m.GameMode = parser.Int(e, "m_pGameRules.m_iGameMode")
		m.LobbyType = parser.Int(e, "m_pGameRules.m_lobbyType")
		m.Winner = parser.Int(e, "m_pGameRules.m_nGameWinner")
		m.GameStartTime = parser.Float(e, "m_pGameRules.m_flGameStartTime")
		m.TotalPausedTks = parser.Int(e, "m_pGameRules.m_nTotalPausedTicks")

	case strings.HasPrefix(cls, "CDOTA_NPC_Observer_Ward"):
		if op.Flag(manta.EntityOpCreated) {
			x, y, _ := parser.WorldPos(e)
			d.wardsByIdx[e.GetIndex()] = len(d.out.Wards)
			d.out.Wards = append(d.out.Wards, wardInfo{
				Class: cls, Created: d.gt(d.p.Tick), X: round2(x), Y: round2(y),
				Team: parser.Int(e, "m_iTeamNum"), OwnerPID: parser.Int(e, "m_nPlayerOwnerID") / 2,
			})
		}
		if op.Flag(manta.EntityOpDeleted) {
			if i, ok := d.wardsByIdx[e.GetIndex()]; ok {
				d.out.Wards[i].Removed = d.gt(d.p.Tick)
				delete(d.wardsByIdx, e.GetIndex())
			}
		}
	}

	// Field schema with values: capture once, at the chosen game time.
	if ci.Fields == nil && (d.full || interesting.MatchString(cls)) && d.clock.Started() &&
		d.clock.GameTime(d.p.Tick) >= d.at {
		ci.Fields = jsonSafe(e.Map())
	}
	return nil
}

// onTick takes per-player snapshots every d.step game seconds after the horn.
func (d *dumper) onTick() {
	if !d.clock.Started() {
		return
	}
	t := d.clock.GameTime(d.p.Tick)
	if t < d.nextSnap {
		return
	}
	d.nextSnap += d.step
	pr := d.find("CDOTA_PlayerResource")
	if pr == nil {
		return
	}
	data := map[int]*manta.Entity{2: d.find("CDOTA_DataRadiant"), 3: d.find("CDOTA_DataDire")}
	for pid := range 24 {
		pfx := fmt.Sprintf("m_vecPlayerTeamData.%04d.", pid)
		if !pr.Exists(pfx + "m_hSelectedHero") {
			break
		}
		team := parser.Int(pr, fmt.Sprintf("m_vecPlayerData.%04d.m_iPlayerTeam", pid))
		hero := d.p.FindEntityByHandle(parser.Handle(pr, pfx+"m_hSelectedHero"))
		if hero == nil || (team != 2 && team != 3) {
			continue
		}
		s := snapshot{GameTime: round2(t), PlayerID: pid}
		x, y, _ := parser.WorldPos(hero)
		s.X, s.Y = round2(x), round2(y)
		s.Level = parser.Int(hero, "m_iCurrentLevel")
		s.HP = parser.Int(hero, "m_iHealth")
		s.Alive = parser.Int(hero, "m_lifeState") == 0
		s.XP = parser.Int(hero, "m_iCurrentXP")
		if de := data[team]; de != nil {
			dp := fmt.Sprintf("m_vecDataTeam.%04d.", parser.Int(pr, pfx+"m_iTeamSlot"))
			s.NetWorth = parser.Int(de, dp+"m_iNetWorth")
			s.Gold = parser.Int(de, dp+"m_iReliableGold") + parser.Int(de, dp+"m_iUnreliableGold")
			s.LH = parser.Int(de, dp+"m_iLastHitCount")
			s.DN = parser.Int(de, dp+"m_iDenyCount")
			s.CreepGold = parser.Int(de, dp+"m_iCreepKillGold")
			s.NeutralGold = parser.Int(de, dp+"m_iNeutralKillGold")
			s.HeroGold = parser.Int(de, dp+"m_iHeroKillGold")
			s.BuildGold = parser.Int(de, dp+"m_iBuildingGold")
			s.IncomeGold = parser.Int(de, dp+"m_iIncomeGold")
			s.BountyGold = parser.Int(de, dp+"m_iBountyGold")
			s.OtherGold = parser.Int(de, dp+"m_iOtherGold")
			s.TotalEarned = parser.Int(de, dp+"m_iTotalEarnedGold")
		}
		d.out.Timeline = append(d.out.Timeline, s)
	}
}

func (d *dumper) find(cls string) *manta.Entity {
	es := d.p.FilterEntity(func(e *manta.Entity) bool { return e.GetClassName() == cls })
	if len(es) == 0 {
		return nil
	}
	return es[0]
}

// finish fills fields that need the whole replay (final scoreboard, game times
// of pre-horn events).
func (d *dumper) finish() {
	d.clock.Close(d.p.Tick)
	d.out.Pauses = d.clock.Pauses()
	d.out.Match.StartTick = d.clock.StartTick()
	for i := range d.out.States {
		s := &d.out.States[i]
		s.GameTime = d.gt(s.Tick)
		if s.State == parser.StatePostGame {
			d.out.Match.EndGameTime = s.GameTime
		}
	}
	for t, evs := range d.rawCL {
		for _, e := range evs {
			e.m["_tick"] = e.tick
			e.m["_game_time"] = d.gt(e.tick)
			d.out.CombatLog[t].Samples = append(d.out.CombatLog[t].Samples, e.m)
		}
	}
	// Keep only the 40 most frequent modifiers.
	type kv struct {
		k string
		v int
	}
	var mods []kv
	for k, v := range d.out.TopModifiers {
		mods = append(mods, kv{k, v})
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].v > mods[j].v })
	d.out.TopModifiers = map[string]int{}
	for i := range min(len(mods), 40) {
		d.out.TopModifiers[mods[i].k] = mods[i].v
	}

	// Final scoreboard / roster from PlayerResource.
	pr := d.find("CDOTA_PlayerResource")
	if pr == nil {
		return
	}
	for pid := range 24 {
		pd := fmt.Sprintf("m_vecPlayerData.%04d.", pid)
		pt := fmt.Sprintf("m_vecPlayerTeamData.%04d.", pid)
		if !pr.Exists(pd + "m_iPlayerSteamID") {
			break
		}
		team := parser.Int(pr, pd+"m_iPlayerTeam")
		if team != 2 && team != 3 {
			continue
		}
		steam, _ := pr.GetUint64(pd + "m_iPlayerSteamID")
		name, _ := pr.GetString(pd + "m_iszPlayerName")
		pi := playerInfo{
			PlayerID: pid, Name: name, Steam64: steam, Team: team,
			TeamSlot: parser.Int(pr, pt+"m_iTeamSlot"),
			HeroID:   parser.Int(pr, pt+"m_nSelectedHeroID"),
			Kills:    parser.Int(pr, pt+"m_iKills"),
			Deaths:   parser.Int(pr, pt+"m_iDeaths"),
			Assists:  parser.Int(pr, pt+"m_iAssists"),
			RankTier: parser.Int(pr, pd+"m_iRankTier"),
			HeroName: d.fileHeroes[steam],
		}
		if steam > steam64Base {
			pi.AccountID = uint32(steam - steam64Base)
		}
		if h := d.p.FindEntityByHandle(parser.Handle(pr, pt+"m_hSelectedHero")); h != nil {
			pi.HeroClass = h.GetClassName()
		}
		d.out.Players = append(d.out.Players, pi)
	}
}

func protoMap(m proto.Message) map[string]any {
	b, _ := protojson.Marshal(m)
	var r map[string]any
	_ = json.Unmarshal(b, &r)
	return r
}

// jsonSafe stringifies values encoding/json can't marshal (NaN etc).
func jsonSafe(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if _, err := json.Marshal(v); err != nil {
			out[k] = fmt.Sprintf("%v", v)
			continue
		}
		out[k] = v
	}
	return out
}

func round2(f float64) float64 { return float64(int64(f*100)) / 100 }
