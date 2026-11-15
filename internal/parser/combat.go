package parser

import (
	"math"
	"sort"
	"strings"

	"github.com/dotabuff/manta/dota"
)

// rawEntry is a combat log entry kept until the end of the parse, when hero
// npc names can be resolved to player ids (file info comes last in a replay).
type rawEntry struct {
	tick         uint32
	typ          dota.DOTA_COMBATLOG_TYPES
	attacker     string // npc name
	source       string // damage source: owner hero for summons / illusions
	target       string
	targetSource string
	inflictor    string // "" for autoattacks / none
	value        int
	attHero      bool
	attIllusion  bool
	tgtHero      bool
	tgtIllusion  bool
	tgtBuilding  bool
	attTeam      int
	tgtTeam      int
	heal         bool // HEAL from regen is dropped at capture
}

func (x *extractor) inflictor(idx uint32) string {
	s := x.name(idx)
	if s == "dota_unknown" {
		return ""
	}
	return s
}

// onCombatLog captures the entries used by the combat, cast, hit and purchase
// extraction. Called for every combat log message.
func (x *extractor) onCombatLog(m *dota.CMsgDOTACombatLogEntry) {
	typ := m.GetType()
	switch typ {
	case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_DAMAGE,
		dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_HEAL,
		dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_MODIFIER_ADD,
		dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ABILITY,
		dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ITEM,
		dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_PURCHASE:
	case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_BUYBACK:
		x.buybacks = append(x.buybacks, tickPID{tick: x.p.Tick, pid: int(m.GetValue())})
		return
	case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_DEATH:
		x.onDeathLog(m)
		return
	default:
		return
	}
	if typ == dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_HEAL && m.GetHealFromRegen() {
		return
	}
	if (typ == dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ABILITY || typ == dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ITEM) &&
		(!m.GetIsAttackerHero() || m.GetIsAttackerIllusion() || m.GetIsAbilityToggleOff()) {
		return
	}
	e := rawEntry{
		tick:         x.p.Tick,
		typ:          typ,
		attacker:     x.name(m.GetAttackerName()),
		source:       x.name(m.GetDamageSourceName()),
		target:       x.name(m.GetTargetName()),
		targetSource: x.name(m.GetTargetSourceName()),
		inflictor:    x.inflictor(m.GetInflictorName()),
		value:        int(m.GetValue()),
		attHero:      m.GetIsAttackerHero(),
		attIllusion:  m.GetIsAttackerIllusion(),
		tgtHero:      m.GetIsTargetHero(),
		tgtIllusion:  m.GetIsTargetIllusion(),
		tgtBuilding:  m.GetIsTargetBuilding(),
		attTeam:      int(m.GetAttackerTeam()),
		tgtTeam:      int(m.GetTargetTeam()),
	}
	switch typ {
	case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_MODIFIER_ADD:
		// Only debuffs on enemy heroes matter (hit heuristic); the ability that
		// applied the modifier is more useful than the modifier name.
		if !e.tgtHero || e.tgtIllusion || e.attTeam == e.tgtTeam || e.attTeam == 0 {
			return
		}
		if a := m.GetModifierAbility(); a != 0 {
			e.inflictor = x.inflictor(a)
		}
		if e.inflictor == "" {
			return
		}
	case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_PURCHASE:
		e.inflictor = x.name(uint32(m.GetValue()))
	}
	x.raw = append(x.raw, e)
}

func (x *extractor) onDeathLog(m *dota.CMsgDOTACombatLogEntry) {
	target := x.name(m.GetTargetName())
	if target == "npc_dota_observer_wards" || target == "npc_dota_sentry_wards" {
		if x.name(m.GetAttackerName()) == target {
			return // expired: the ward "kills" itself
		}
		x.wardDeaths = append(x.wardDeaths, wardDeath{
			tick:     x.p.Tick,
			sentry:   target == "npc_dota_sentry_wards",
			team:     int(m.GetTargetTeam()),
			attacker: x.name(m.GetAttackerName()),
			source:   x.name(m.GetDamageSourceName()),
		})
		return
	}
	if !m.GetIsTargetHero() || m.GetIsTargetIllusion() || m.GetWillReincarnate() {
		return
	}
	assists := make([]int, 0, len(m.GetAssistPlayers()))
	for _, a := range m.GetAssistPlayers() {
		assists = append(assists, int(a))
	}
	x.rawDeaths = append(x.rawDeaths, rawDeath{
		tick:      x.p.Tick,
		victim:    target,
		killer:    x.name(m.GetAttackerName()),
		source:    x.name(m.GetDamageSourceName()),
		inflictor: x.inflictor(m.GetInflictorName()),
		assists:   assists,
	})
}

// unitCategory groups non-hero units for combat aggregates.
func unitCategory(npc string, building bool) string {
	switch {
	case building || strings.Contains(npc, "tower") || strings.Contains(npc, "rax") ||
		strings.Contains(npc, "fort") || strings.Contains(npc, "_healers") || strings.Contains(npc, "outpost"):
		return "building"
	case strings.HasPrefix(npc, "npc_dota_creep_neutral") || strings.HasPrefix(npc, "npc_dota_neutral_"):
		return "neutral"
	case strings.HasPrefix(npc, "npc_dota_creep") || strings.HasPrefix(npc, "npc_dota_goodguys_siege") ||
		strings.HasPrefix(npc, "npc_dota_badguys_siege"):
		return "creep"
	case strings.HasPrefix(npc, "npc_dota_roshan"):
		return "roshan"
	case strings.HasSuffix(npc, "_wards"):
		return "ward"
	}
	return "unit"
}

// owner resolves the player that controls an attacker (hero, illusion or summon).
func owner(pidByHero map[string]int, unit, source string) (int, bool) {
	if pid, ok := pidByHero[unit]; ok {
		return pid, true
	}
	pid, ok := pidByHero[source]
	return pid, ok
}

type combatKey struct {
	pid                    int
	kind, other, inflictor string
}

// buildCombat turns the raw combat log into aggregates, casts, hits,
// purchases and cumulative per-player damage (for the series).
func (x *extractor) buildCombat(pidByHero map[string]int) {
	heroByPID := map[int]string{}
	for h, pid := range pidByHero {
		heroByPID[pid] = h
	}
	agg := map[combatKey]*CombatStat{}
	add := func(pid int, kind, other, inflictor string, v int) {
		k := combatKey{pid, kind, other, inflictor}
		cs := agg[k]
		if cs == nil {
			cs = &CombatStat{PlayerID: pid, Kind: kind, Other: other, Inflictor: inflictor}
			agg[k] = cs
		}
		cs.Amount += v
		cs.Count++
	}
	// Cumulative damage per player, as (tick, total) steps for the series.
	dmgOut := map[int][]tickValue{}
	dmgIn := map[int][]tickValue{}
	bump := func(m map[int][]tickValue, pid int, tick uint32, v int) {
		s := m[pid]
		total := v
		if n := len(s); n > 0 {
			total += s[n-1].v
			if s[n-1].tick == tick {
				s[n-1].v = total
				return
			}
		}
		m[pid] = append(s, tickValue{tick, total})
	}
	lastHit := map[combatKey]float64{}
	hit := func(pid int, ability string, tick uint32) {
		t := x.clock.GameTime(tick)
		k := combatKey{pid: pid, inflictor: ability}
		if last, ok := lastHit[k]; ok && t-last < HitMerge {
			return
		}
		lastHit[k] = t
		x.m.Hits = append(x.m.Hits, Hit{T: round2(t), PlayerID: pid, Ability: ability})
	}
	// other side of an entry, as seen from the player.
	targetName := func(e rawEntry) string {
		switch {
		case e.tgtIllusion:
			return "illusion"
		case e.tgtHero:
			return e.target
		}
		return unitCategory(e.target, e.tgtBuilding)
	}

	for _, e := range x.raw {
		switch e.typ {
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_DAMAGE:
			if pid, ok := owner(pidByHero, e.attacker, e.source); ok && e.attTeam != e.tgtTeam {
				add(pid, DmgOut, targetName(e), e.inflictor, e.value)
				bump(dmgOut, pid, e.tick, e.value)
				if e.tgtHero && !e.tgtIllusion && e.inflictor != "" {
					hit(pid, e.inflictor, e.tick)
				}
			}
			if e.tgtHero && !e.tgtIllusion {
				if pid, ok := pidByHero[e.target]; ok {
					other := unitCategory(e.attacker, false)
					if o, ok := owner(pidByHero, e.attacker, e.source); ok {
						other = heroByPID[o]
					}
					add(pid, DmgIn, other, e.inflictor, e.value)
					bump(dmgIn, pid, e.tick, e.value)
				}
			}
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_HEAL:
			if pid, ok := owner(pidByHero, e.attacker, e.source); ok && e.value > 0 {
				add(pid, HealOut, targetName(e), e.inflictor, e.value)
			}
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_MODIFIER_ADD:
			if pid, ok := owner(pidByHero, e.attacker, e.source); ok {
				hit(pid, e.inflictor, e.tick)
			}
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ABILITY, dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ITEM:
			if strings.HasPrefix(e.inflictor, "plus_") || e.inflictor == "" {
				continue // high fives, guild banners and other cosmetics
			}
			if pid, ok := pidByHero[e.attacker]; ok {
				x.m.Casts = append(x.m.Casts, Cast{T: round2(x.clock.GameTime(e.tick)), PlayerID: pid, Ability: e.inflictor})
			}
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_PURCHASE:
			item := strings.TrimPrefix(e.inflictor, "item_")
			if item == "" || item == "ward_dispenser" || strings.HasPrefix(item, "recipe_") {
				continue // stack merges and recipes: the assembled item is logged too
			}
			if pid, ok := pidByHero[e.target]; ok {
				x.m.Purchases = append(x.m.Purchases, Purchase{
					T: round2(x.clock.GameTime(e.tick)), PlayerID: pid, Item: item,
				})
			}
		}
	}
	x.raw = nil

	for _, cs := range agg {
		x.m.Combat = append(x.m.Combat, *cs)
	}
	sort.Slice(x.m.Combat, func(i, j int) bool {
		a, b := x.m.Combat[i], x.m.Combat[j]
		if a.PlayerID != b.PlayerID {
			return a.PlayerID < b.PlayerID
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Amount > b.Amount
	})
	sort.SliceStable(x.m.Hits, func(i, j int) bool { return x.m.Hits[i].T < x.m.Hits[j].T })

	// Fill the cumulative damage columns of the series.
	at := func(steps []tickValue, t float64) int {
		i := sort.Search(len(steps), func(i int) bool { return x.clock.GameTime(steps[i].tick) > t })
		if i == 0 {
			return 0
		}
		return steps[i-1].v
	}
	for i := range x.m.Series {
		s := &x.m.Series[i]
		s.DmgOut = at(dmgOut[s.PlayerID], s.T)
		s.DmgIn = at(dmgIn[s.PlayerID], s.T)
	}
}

type tickValue struct {
	tick uint32
	v    int
}

type tickPID struct {
	tick uint32
	pid  int
}

type wardDeath struct {
	tick             uint32
	sentry           bool
	team             int
	attacker, source string
}

type rawWard struct {
	created, removed uint32
	died             uint32 // tick the life state left "alive" (killed or expired)
	sentry           bool
	team             int
	ownerHandle      uint64
	ownerPID         int // m_nPlayerOwnerID fallback
	x, y             float64
}

// buildWards resolves ward owners and matches ward deaths to ward entities.
func (x *extractor) buildWards(pidByHero map[string]int, pidByHandle map[uint64]int) {
	used := make([]bool, len(x.wardDeaths))
	const window = TickRate
	for _, w := range x.wards {
		pid, ok := pidByHandle[w.ownerHandle]
		if !ok {
			pid = w.ownerPID
		}
		removed := w.died
		if removed == 0 {
			removed = w.removed
		}
		if removed == 0 {
			removed = x.p.Tick
		}
		out := Ward{
			T: round2(x.clock.GameTime(w.created)), End: round2(x.clock.GameTime(removed)),
			PlayerID: pid, Team: w.team, Sentry: w.sentry, X: round1(w.x), Y: round1(w.y), KillerPID: -1,
		}
		if w.died != 0 {
			best, bestDist := -1, uint32(math.MaxUint32)
			for i, d := range x.wardDeaths {
				if used[i] || d.sentry != w.sentry || (d.team != 0 && d.team != w.team) {
					continue
				}
				if dist := absDiff(w.died, d.tick); dist <= window && dist < bestDist {
					best, bestDist = i, dist
				}
			}
			if best >= 0 {
				used[best] = true
				d := x.wardDeaths[best]
				out.Destroyed, out.Killer = true, d.attacker
				if k, ok := owner(pidByHero, d.attacker, d.source); ok {
					out.KillerPID = k
				}
			}
		}
		x.m.Wards = append(x.m.Wards, out)
	}
}
