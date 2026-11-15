package parser

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/dotabuff/manta"
	"github.com/dotabuff/manta/dota"
)

const maxPlayers = 24 // PlayerResource vector size; real players are 0..9

// ParseFile parses a replay file. It never panics: manta failures are returned
// as *ParseError.
func ParseFile(path string) (*Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse extracts a Match from a replay stream. It never panics.
func Parse(r io.Reader) (*Match, error) {
	x := &extractor{
		lastState: -1,
		lastLife:  map[int]int{},
		heroByPID: map[int]string{},
	}
	err := Safe(func() error {
		p, err := manta.NewStreamParser(r)
		if err != nil {
			return err
		}
		x.p = p
		x.register()
		if err := p.Start(); err != nil {
			return err
		}
		x.m.GameBuild = p.GameBuild
		x.finish()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !x.clock.Started() {
		return nil, &ParseError{Msg: "game never reached GAME_IN_PROGRESS"}
	}
	if len(x.m.Players) == 0 {
		return nil, &ParseError{Msg: "no players found in replay"}
	}
	return &x.m, nil
}

type rawDeath struct {
	tick      uint32
	victim    string // npc name
	killer    string // attacker npc name
	source    string // damage source (owner hero for summons)
	inflictor string
	assists   []int
}

type posDeath struct {
	tick uint32
	pid  int
	x, y float64
}

type extractor struct {
	p     *manta.Parser
	clock Clock
	m     Match

	lastState int
	postGame  uint32

	rules, pr       *manta.Entity
	dataTeam        [4]*manta.Entity // index by team (2, 3)
	nextSample      float64
	lastLife        map[int]int // pid -> lifeState
	rawDeaths       []rawDeath
	posDeaths       []posDeath
	heroByPID       map[int]string // npc_dota_hero_* by player id (from file info)
	fileHeroBySteam map[uint64]string
	fileHeroOrdered []string
	lastTickSampled uint32
}

func (x *extractor) name(idx uint32) string {
	s, _ := x.p.LookupStringByIndex("CombatLogNames", int32(idx))
	return s
}

func (x *extractor) register() {
	p := x.p
	p.Callbacks.OnCDemoFileInfo(func(m *dota.CDemoFileInfo) error {
		gi := m.GetGameInfo().GetDota()
		if gi.GetMatchId() != 0 {
			x.m.MatchID = gi.GetMatchId()
		}
		x.m.EndTime = int64(gi.GetEndTime())
		if w := int(gi.GetGameWinner()); w == TeamRadiant || w == TeamDire {
			x.m.Winner = w
		}
		x.fileHeroBySteam = map[uint64]string{}
		for _, pi := range gi.GetPlayerInfo() {
			x.fileHeroBySteam[pi.GetSteamid()] = pi.GetHeroName()
			x.fileHeroOrdered = append(x.fileHeroOrdered, pi.GetHeroName())
		}
		return nil
	})

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		if op.Flag(manta.EntityOpCreated) {
			switch e.GetClassName() {
			case "CDOTAGamerulesProxy":
				x.rules = e
			case "CDOTA_PlayerResource":
				x.pr = e
			case "CDOTA_DataRadiant":
				x.dataTeam[TeamRadiant] = e
			case "CDOTA_DataDire":
				x.dataTeam[TeamDire] = e
			}
		}
		if e == x.rules && !op.Flag(manta.EntityOpDeleted) {
			x.onRules(e)
		}
		return nil
	})

	p.Callbacks.OnCMsgDOTACombatLogEntry(func(m *dota.CMsgDOTACombatLogEntry) error {
		if m.GetType() != dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_DEATH ||
			!m.GetIsTargetHero() || m.GetIsTargetIllusion() || m.GetWillReincarnate() {
			return nil
		}
		inf := x.name(m.GetInflictorName())
		if inf == "dota_unknown" {
			inf = ""
		}
		assists := make([]int, 0, len(m.GetAssistPlayers()))
		for _, a := range m.GetAssistPlayers() {
			assists = append(assists, int(a))
		}
		x.rawDeaths = append(x.rawDeaths, rawDeath{
			tick:      x.p.Tick,
			victim:    x.name(m.GetTargetName()),
			killer:    x.name(m.GetAttackerName()),
			source:    x.name(m.GetDamageSourceName()),
			inflictor: inf,
			assists:   assists,
		})
		return nil
	})

	p.Callbacks.OnCNETMsg_Tick(func(*dota.CNETMsg_Tick) error {
		x.onTick()
		return nil
	})
}

func (x *extractor) onRules(e *manta.Entity) {
	x.clock.SetPaused(x.p.Tick, Int(e, "m_pGameRules.m_bGamePaused") == 1)
	st := Int(e, "m_pGameRules.m_nGameState")
	if st != x.lastState {
		x.lastState = st
		switch st {
		case StateGameInProgress:
			x.clock.MarkStart(x.p.Tick)
		case StatePostGame:
			if x.postGame == 0 {
				x.postGame = x.p.Tick
			}
		}
	}
	x.m.GameMode = Int(e, "m_pGameRules.m_iGameMode")
	x.m.LobbyType = Int(e, "m_pGameRules.m_lobbyType")
	if w := Int(e, "m_pGameRules.m_nGameWinner"); w == TeamRadiant || w == TeamDire {
		x.m.Winner = w
	}
}

// heroOf returns the hero entity selected by player pid.
func (x *extractor) heroOf(pid int) *manta.Entity {
	h := Handle(x.pr, fmt.Sprintf("m_vecPlayerTeamData.%04d.m_hSelectedHero", pid))
	if h == NullHandle {
		return nil
	}
	return x.p.FindEntityByHandle(h)
}

// players returns pids of real players (team radiant/dire) currently in PlayerResource.
func (x *extractor) players() []int {
	var out []int
	for pid := range maxPlayers {
		key := fmt.Sprintf("m_vecPlayerData.%04d.m_iPlayerTeam", pid)
		if !x.pr.Exists(key) {
			break
		}
		if t := Int(x.pr, key); t == TeamRadiant || t == TeamDire {
			out = append(out, pid)
		}
	}
	return out
}

func (x *extractor) onTick() {
	if x.pr == nil || x.postGame != 0 || x.p.Tick == x.lastTickSampled {
		return
	}
	x.lastTickSampled = x.p.Tick
	// Life states are tracked from the very start (first blood can happen
	// before the horn); samples are taken from 00:00 on.
	t := x.clock.GameTime(x.p.Tick)
	sample := x.clock.Started() && t >= x.nextSample
	if sample {
		x.nextSample = math.Floor(t/SeriesStep)*SeriesStep + SeriesStep
	}
	for _, pid := range x.players() {
		hero := x.heroOf(pid)
		if hero == nil {
			continue
		}
		life := Int(hero, "m_lifeState")
		if prev, ok := x.lastLife[pid]; ok && prev == 0 && life != 0 {
			px, py, _ := WorldPos(hero)
			x.posDeaths = append(x.posDeaths, posDeath{tick: x.p.Tick, pid: pid, x: px, y: py})
		}
		x.lastLife[pid] = life
		if !sample {
			continue
		}
		s := Sample{T: math.Round(t/SeriesStep) * SeriesStep, PlayerID: pid}
		s.X, s.Y, _ = WorldPos(hero)
		s.Alive = life == 0
		s.Level = Int(hero, "m_iCurrentLevel")
		x.fillEconomy(pid, func(de *manta.Entity, dp string) {
			s.Gold = Int(de, dp+"m_iReliableGold") + Int(de, dp+"m_iUnreliableGold")
			s.NetWorth = Int(de, dp+"m_iNetWorth")
			s.TotalGold = Int(de, dp+"m_iTotalEarnedGold")
			s.TotalXP = Int(de, dp+"m_iTotalEarnedXP")
			s.LastHits = Int(de, dp+"m_iLastHitCount")
			s.Denies = Int(de, dp+"m_iDenyCount")
		})
		s.X, s.Y = round1(s.X), round1(s.Y)
		x.m.Series = append(x.m.Series, s)
	}
}

// fillEconomy calls fn with the team data entity and field prefix of player pid.
func (x *extractor) fillEconomy(pid int, fn func(de *manta.Entity, prefix string)) {
	team := Int(x.pr, fmt.Sprintf("m_vecPlayerData.%04d.m_iPlayerTeam", pid))
	if team != TeamRadiant && team != TeamDire || x.dataTeam[team] == nil {
		return
	}
	slot := Int(x.pr, fmt.Sprintf("m_vecPlayerTeamData.%04d.m_iTeamSlot", pid))
	fn(x.dataTeam[team], fmt.Sprintf("m_vecDataTeam.%04d.", slot))
}

func (x *extractor) finish() {
	x.clock.Close(x.p.Tick)
	x.m.StartTick = x.clock.StartTick()
	end := x.postGame
	if end == 0 {
		end = x.p.Tick
	}
	x.m.Duration = x.clock.GameTime(end)
	if x.pr == nil {
		return
	}

	pidByHero := map[string]int{}
	for i, pid := range x.players() {
		pd := fmt.Sprintf("m_vecPlayerData.%04d.", pid)
		pt := fmt.Sprintf("m_vecPlayerTeamData.%04d.", pid)
		steam, _ := x.pr.GetUint64(pd + "m_iPlayerSteamID")
		name, _ := x.pr.GetString(pd + "m_iszPlayerName")
		pl := Player{
			PlayerID:  pid,
			Team:      Int(x.pr, pd+"m_iPlayerTeam"),
			TeamSlot:  Int(x.pr, pt+"m_iTeamSlot"),
			HeroID:    Int(x.pr, pt+"m_nSelectedHeroID"),
			Name:      name,
			AccountID: AccountFromSteam64(steam),
			Kills:     Int(x.pr, pt+"m_iKills"),
			Deaths:    Int(x.pr, pt+"m_iDeaths"),
			Assists:   Int(x.pr, pt+"m_iAssists"),
			Level:     Int(x.pr, pt+"m_iLevel"),
			RankTier:  Int(x.pr, pd+"m_iRankTier"),
		}
		pl.HeroName = x.fileHeroBySteam[steam]
		if pl.HeroName == "" && steam == 0 && i < len(x.fileHeroOrdered) {
			pl.HeroName = x.fileHeroOrdered[i] // bots: file info keeps player order
		}
		x.fillEconomy(pid, func(de *manta.Entity, dp string) {
			pl.LastHits = Int(de, dp+"m_iLastHitCount")
			pl.Denies = Int(de, dp+"m_iDenyCount")
			pl.NetWorth = Int(de, dp+"m_iNetWorth")
			pl.TotalGold = Int(de, dp+"m_iTotalEarnedGold")
			pl.TotalXP = Int(de, dp+"m_iTotalEarnedXP")
			pl.HeroDamage = Int(de, dp+"m_flHeroDamage")
			pl.TowerDamage = Int(de, dp+"m_flTowerDamage")
			pl.Healing = Int(de, dp+"m_fHealing")
		})
		if pl.HeroName != "" {
			pidByHero[pl.HeroName] = pid
		}
		x.m.Players = append(x.m.Players, pl)
	}
	x.buildDeaths(pidByHero)
}

// buildDeaths resolves combat-log deaths to player ids and attaches the
// position of the victim hero at the moment its life state flipped.
func (x *extractor) buildDeaths(pidByHero map[string]int) {
	used := make([]bool, len(x.posDeaths))
	const window = 2 * TickRate // ticks
	for _, rd := range x.rawDeaths {
		victim, ok := pidByHero[rd.victim]
		if !ok {
			continue
		}
		d := Death{
			T:         round2(x.clock.GameTime(rd.tick)),
			VictimPID: victim,
			KillerPID: -1,
			Killer:    rd.killer,
			Inflictor: rd.inflictor,
			Assists:   rd.assists,
		}
		if k, ok := pidByHero[rd.killer]; ok {
			d.KillerPID = k
		} else if k, ok := pidByHero[rd.source]; ok {
			d.KillerPID = k // summon / illusion of a hero
		}
		if d.KillerPID >= 0 { // the combat log lists the killer among assists
			kept := d.Assists[:0]
			for _, a := range d.Assists {
				if a != d.KillerPID {
					kept = append(kept, a)
				}
			}
			d.Assists = kept
		}
		best, bestDist := -1, uint32(math.MaxUint32)
		for i, pd := range x.posDeaths {
			if used[i] || pd.pid != victim {
				continue
			}
			dist := absDiff(pd.tick, rd.tick)
			if dist <= window && dist < bestDist {
				best, bestDist = i, dist
			}
		}
		if best >= 0 {
			used[best] = true
			d.X, d.Y, d.HasPos = round1(x.posDeaths[best].x), round1(x.posDeaths[best].y), true
		}
		x.m.Deaths = append(x.m.Deaths, d)
	}
	sort.SliceStable(x.m.Deaths, func(i, j int) bool { return x.m.Deaths[i].T < x.m.Deaths[j].T })
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }
func round2(f float64) float64 { return math.Round(f*100) / 100 }
