package stats

import (
	"math"
	"sort"
	"strings"

	"github.com/1rowvy/drstats/internal/constants"
	"github.com/1rowvy/drstats/internal/parser"
)

// Fight play thresholds.
const (
	fightLate      = 10.0 // seconds after the start the first damage counts as joining late
	fightFar       = 2500 // distance to the fight at its start that counts as far away
	ultLevel       = 6    // level an ultimate is assumed learned from
	ultCooldownMin = 70.0 // abilities with a cooldown this long count as "big" spells
	keyCastLead    = 2.0  // a key spell cast this long before the start still counts for the fight
)

// saveItems are the active items whose use (or not) decides a fight for
// their owner.
var saveItems = map[string]bool{
	"item_black_king_bar": true, "item_manta": true, "item_satanic": true, "item_ghost": true,
	"item_cyclone": true, "item_wind_waker": true, "item_glimmer_cape": true, "item_force_staff": true,
	"item_hurricane_pike": true, "item_lotus_orb": true, "item_blade_mail": true, "item_pipe": true,
	"item_crimson_guard": true, "item_guardian_greaves": true, "item_refresher": true,
	"item_ethereal_blade": true, "item_disperser": true, "item_shivas_guard": true, "item_sheepstick": true,
	"item_abyssal_blade": true, "item_bloodthorn": true, "item_orchid": true, "item_nullifier": true,
}

// isKeySpell: an ultimate-like ability (three cooldown levels or a long
// cooldown) or a saving / disabling item.
func isKeySpell(key string, cd constants.Cooldown) bool {
	if cd.Item {
		return saveItems[key]
	}
	return cd.Levels == 3 || cd.Max >= ultCooldownMin
}

// FightPlay is how the focused player played one fight.
type FightPlay struct {
	// Who opened the fight: the first ability hit on a hero (else the
	// biggest damage dealer of the opening bucket); -1 when unknown.
	Initiator   int    `json:"initiator"`
	InitAbility string `json:"init_ability"` // "" = autoattacks / unknown
	WeStarted   bool   `json:"we_started"`
	// FirstTarget took the most damage in the opening 5 s; -1 unknown.
	FirstTarget int `json:"first_target"`
	// MyEntry: seconds from the start to my first damage; nil = none.
	MyEntry *float64 `json:"my_entry"`
	MyDist  float64  `json:"my_dist"` // my distance to the fight when it started
	Late    bool     `json:"late"`
	// Key spells (ultimates, saving items): used in the fight, and ready
	// but not used before I died (or by the end of the fight).
	KeyUsed   []string `json:"key_used"`
	KeyUnused []string `json:"key_unused"`
	// DeathAfter: seconds from the start to my death in this fight; nil = survived.
	DeathAfter *float64 `json:"death_after"`
}

// fightPlay analyses the focused player's part in a fight window.
func fightPlay(m *parser.Match, b *Battle, from, to float64, players []parser.Player, byPID map[int][]parser.Sample,
	focus int, cds map[string]constants.Cooldown) FightPlay {
	fp := FightPlay{Initiator: -1, FirstTarget: -1, KeyUsed: []string{}, KeyUnused: []string{}}
	team := teamOf(players, focus)
	start := from
	// Opening hit.
	for _, h := range m.Hits {
		if h.T >= from && h.T < to && (fp.Initiator < 0 || h.T < start) {
			fp.Initiator, fp.InitAbility, start = h.PlayerID, h.Ability, h.T
		}
	}
	// The opening bucket: who dealt and who took the most.
	opening := math.Floor(start/parser.DamageBucketStep) * parser.DamageBucketStep
	dealt, taken := map[int]int{}, map[int]int{}
	for _, d := range m.HeroDamage {
		if d.T == opening {
			dealt[d.PlayerID] += d.Amount
			taken[d.Target] += d.Amount
		}
	}
	if fp.Initiator < 0 {
		fp.Initiator = argmax(dealt)
	}
	fp.FirstTarget = argmax(taken)
	if fp.Initiator >= 0 {
		fp.WeStarted = teamOf(players, fp.Initiator) == team
	}

	// When I joined.
	entry := math.Inf(1)
	for _, h := range m.Hits {
		if h.PlayerID == focus && h.T >= start && h.T < to {
			entry = math.Min(entry, h.T)
		}
	}
	for _, d := range m.HeroDamage {
		if d.PlayerID == focus && d.T >= from && d.T < to && d.Amount > 0 {
			entry = math.Min(entry, math.Max(d.T, start))
		}
	}
	if !math.IsInf(entry, 1) {
		e := entry - start
		fp.MyEntry = &e
		fp.Late = e >= fightLate
	}
	if s, ok := sampleAt(byPID[focus], math.Floor(start/parser.SeriesStep)*parser.SeriesStep); ok && s.Alive {
		fp.MyDist = math.Hypot(s.X-b.X, s.Y-b.Y)
	}

	// Key spells: until my death in the fight, else the fight end.
	until := to
	for _, i := range b.Deaths {
		if d := m.Deaths[i]; d.VictimPID == focus {
			until = math.Min(until, d.T)
		}
	}
	if until < to {
		d := math.Max(0, until-start)
		fp.DeathAfter = &d
	}
	level := 0
	if s, ok := sampleAt(byPID[focus], math.Floor(start/parser.SeriesStep)*parser.SeriesStep); ok {
		level = s.Level
	}
	// Own abilities only (Rubick's stolen spells and the like are not his
	// to plan around): ability keys start with the hero's short name.
	var hero string
	for _, p := range players {
		if p.PlayerID == focus {
			hero = strings.TrimPrefix(p.HeroName, "npc_dota_hero_") + "_"
		}
	}
	var casts []parser.Cast
	known := map[string]bool{}
	for _, c := range m.Casts {
		if c.PlayerID != focus {
			continue
		}
		if !strings.HasPrefix(c.Ability, "item_") && !strings.HasPrefix(c.Ability, hero) {
			continue
		}
		if cd, ok := cds[c.Ability]; ok && isKeySpell(c.Ability, cd) {
			casts = append(casts, c)
			known[c.Ability] = true
		}
	}
	bought := map[string]float64{}
	for _, p := range m.Purchases {
		key := "item_" + p.Item
		if p.PlayerID != focus || !saveItems[key] {
			continue
		}
		if _, ok := bought[key]; !ok {
			bought[key] = p.T
		}
		known[key] = true
	}
	keys := make([]string, 0, len(known))
	for k := range known {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cd := cds[k]
		used, last, first := false, math.Inf(-1), math.Inf(1)
		for _, c := range casts {
			if c.Ability != k {
				continue
			}
			first = math.Min(first, c.T)
			if c.T >= start-keyCastLead && c.T <= until {
				used = true
			}
			if c.T < start-keyCastLead {
				last = math.Max(last, c.T)
			}
		}
		if used {
			fp.KeyUsed = append(fp.KeyUsed, k)
			continue
		}
		var ready bool
		if cd.Item || saveItems[k] {
			t, ok := bought[k]
			ready = ok && t < start && last+cd.Max <= start
		} else if !math.IsInf(last, -1) {
			ready = last+cd.Max <= start
		} else if cd.Levels == 3 {
			// Never cast before: assume an ultimate is learned at level 6
			// when it gets used later in the game. Other long-cooldown
			// spells (shard / Aghanim's) may not exist yet: unknown.
			ready = level >= ultLevel && first > until
		}
		if ready {
			fp.KeyUnused = append(fp.KeyUnused, k)
		}
	}
	return fp
}

// argmax returns the key with the largest positive value, -1 if none.
func argmax(m map[int]int) int {
	best, bv := -1, 0
	for k, v := range m {
		if v > bv || (v == bv && v > 0 && k < best) {
			best, bv = k, v
		}
	}
	return best
}
