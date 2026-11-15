# Phase 1 spike — what a replay actually contains

Test replay: match 9091456344, All Pick ranked (game_mode 22, lobby_type 7), 31:47,
build 6975, manta v1.5.2. Full parse of a 64 MB `.dem` takes **~1.2 s**.
Cross-checked against OpenDota's parsed data for the same match: LH, total earned
gold and XP at 10:00 match **exactly** for all 10 players; observer/sentry counts
per player match; duration (1907 s) matches.

Run: `go run ./cmd/dump testdata/<file>.dem > dump.json` (`-samples`, `-at`, `-step`, `-full`).

## Time
- Tick rate 30. Game rules (`CDOTAGamerulesProxy.m_pGameRules`) give `m_nGameState`,
  `m_bGamePaused`, `m_nTotalPausedTicks`, `m_flGameStartTime` (server seconds).
- States seen: HERO_SELECTION → STRATEGY_TIME → TEAM_SHOWCASE → WAIT_FOR_MAP_TO_LOAD →
  PRE_GAME (−90 s) → GAME_IN_PROGRESS (0) → POST_GAME → DISCONNECT.
- `parser.Clock` anchors 00:00 at the GAME_IN_PROGRESS tick and excludes paused ticks.
  Combat log `timestamp − m_flGameStartTime` equals clock time (verified, no pauses in
  this replay — pause handling is unit-tested only).

## Identity
- `CDemoFileInfo.game_info.dota`: match id, mode, winner, end time (unix), per player
  hero name (`npc_dota_hero_*`), name, **steam64**, team. Account id = steam64 − 76561197960265728.
- `CDOTA_PlayerResource.m_vecPlayerData.N` (N = player id 0–9): steam id, name, team, rank tier;
  `m_vecPlayerTeamData.N`: hero id, `m_hSelectedHero` (→ hero entity), K/D/A, level, team slot,
  last buyback time, teamfight participation.
- Gotcha: hero `m_iPlayerID` and ward `m_nPlayerOwnerID` are **2 × player id** in this build.
  Use `m_hSelectedHero` for hero ↔ player mapping.

## Economy & farm (per player, cumulative, sampled any time)
`CDOTA_DataRadiant/Dire.m_vecDataTeam.<team slot>`: net worth, reliable/unreliable gold,
total earned gold & XP, LH, denies, misses, gold by source (creep, neutral, hero, building,
income/passive, bounty runes, roshan, courier, ward kill, other, comeback), gold lost to death,
gold spent (items / consumables / support / buybacks), healing, hero damage, tower damage,
stuns, camps stacked, wards placed/purchased/destroyed, rune pickups, smokes, lotuses, watchers,
TP scrolls, starting position.
→ GPM/XPM/NW series and the "gold sources" chart come straight from these counters.

## Hero entity (`CDOTA_Unit_Hero_*`)
Position (`CBodyComponent.m_cell{X,Y}` × 128 + `m_vec{X,Y}` − 16384 → world units),
level, XP, HP/max HP, mana, stats, damage, life state, death/respawn time, abilities and
items (handles). Gives the movement heatmap, death positions, lane detection.

## Combat log (19 types seen)
DAMAGE 33k, MODIFIER_STACK_EVENT 15k, MODIFIER_ADD/REMOVE 8.6k each, HEAL 4.3k, XP 3.3k,
DEATH 2.5k, ABILITY 2.2k, GOLD 2.2k, ITEM 856, PURCHASE 403, PLAYERSTATS, CRITICAL_DAMAGE,
TEAM_BUILDING_KILL, KILLSTREAK, MULTIKILL, FIRST_BLOOD, BUYBACK, GAME_STATE.
Names resolve via the `CombatLogNames` string table (attacker, target, inflictor = ability/item/modifier).
- DAMAGE: attacker, target, inflictor (`dota_unknown` = autoattack), value, hero/illusion/building flags.
- DEATH: killer, inflictor of killing blow, assist players, target networth. **No position** →
  take hero entity position at the death tick.
- ABILITY / ITEM: every cast / active-item use with ability level.
- PURCHASE: `value` is a CombatLogNames index of the item (`item_quelling_blade`).
- MODIFIER_ADD: debuffs with inflictor → hit-rate heuristic (cast → damage/debuff within N s).
- BUYBACK: player id, time. Ward kills: DEATH of `npc_dota_{observer,sentry}_wards` with attacker.

## Other sources
- Ward entities `CDOTA_NPC_Observer_Ward[_TrueSight]`: create/delete ticks → placement time,
  position, owner, lifetime (destroyed early = dewarded).
- Chat events: RUNE_PICKUP (rune type + player), HERO_KILL, TOWER_KILL, ROSHAN_KILL, AEGIS,
  BUYBACK, SMOKE_ACTIVATED, glyph/scan, ward-killed messages.
- SpectatorPlayerUnitOrders (68k): every player order — APM, possibly "idle" detection.
- Game events: almost empty (`dota_chase_hero`, `hltv_versioninfo`) — not useful.

## Not in the replay (need outside data or heuristics)
- Human names of heroes/items/abilities, item costs → OpenDota constants.
- MMR / rank change, party info, patch name (only numeric build).
- "Hit" of an ability as such — only the heuristic via damage/modifiers.
- Exact lane assignment — must be inferred from early positions (`m_iStartingPosition`
  does not match the real lane reliably).
- Rune pickup is not in the combat log — use the chat event.
- Download: Valve's `.dem.bz2` is currently **zstd**-compressed despite the name.

## Phase 3 findings
- **Starting items bought during strategy time are not in the combat log** (it starts at
  PRE_GAME), so the purchase log misses them; everything after matches OpenDota exactly.
  The combat log also has `ward_dispenser` (ward stack merge) and `recipe_*` entries — skipped.
- Ward deaths: the entity is deleted ~7 s after the DEATH entry; use the `m_lifeState`
  flip. Expired wards also produce a DEATH entry whose attacker is the ward itself.
- Ward owner: `m_hOwnerEntity` is the owner's hero handle (matches `m_hSelectedHero`).
- BUYBACK combat log `value` is the player id.
- MODIFIER_ADD carries `modifier_ability` (CombatLogNames index of the ability that
  applied it) — the right key for the hit heuristic.
- Gold by source: `m_iCreepKillGold + m_iCreepDenyGold`, `m_iNeutralKillGold`,
  `m_iHeroKillGold`, `m_iBuildingGold`, `m_iIncomeGold`, `m_iBountyGold`, `m_iRoshanGold`;
  the rest (courier, wards, abilities, comeback, shared) is "other". `m_iGoldLostToDeath`
  increments right after a death; summed per death it equals the final counter.
- Runes: ChatEvent RUNE_PICKUP / RUNE_BOTTLE, `playerid_1` + rune type in `value`;
  counts match OpenDota `runes_log`. Camp stacks: `m_iCampsStacked` increments.
