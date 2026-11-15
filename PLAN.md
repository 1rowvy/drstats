# drstats — plan

Personal desktop app: parse Dota 2 replays (`.dem`) and show detailed stats of *my*
games on any hero — per match and aggregated. Working name `drstats`.

## Decisions
| Topic | Decision | Why |
|---|---|---|
| Desktop framework | **Wails v3** (v3.1.x) | v3.0.0 went stable on 2026-10-26, so per spec v3 over v2. Multi-window, services, built-in updater. |
| Linux toolkit | GTK4 + WebKitGTK 6.0 (v3 default) | Fedora 43 ships both; GTK3 path is deprecated in v3.1. |
| Replay parser | `github.com/dotabuff/manta` | spec |
| SQLite driver | `modernc.org/sqlite` | Pure Go: no CGO for the DB, Windows cross-compile from Fedora and CI stay simple. Speed difference vs `mattn/go-sqlite3` is irrelevant for one user. |
| Charts | ECharts (`echarts-for-react`) | Handles big time series, zoom, many series. |
| Heatmaps / map | raw `<canvas>` | spec |
| Hero/item/ability names | OpenDota constants (`odota/dotaconstants` JSON), cached in SQLite/app data dir, refresh button | spec |
| Auto-update | Wails v3 updater, GitHub Releases provider; CI builds on tag `v*` | One user, two machines — new build lands on both automatically. Repo is **public** so the updater needs no token. |
| UI language | Russian | Single Russian-speaking user; code, comments and docs stay English. |
| Charts wrapper | own ~30-line wrapper over tree-shaken `echarts/core` | `echarts-for-react` failed to bundle with Vite 8/Rolldown (missing `tslib`); one dependency less. |
| Hero icons | initials on attribute colour, no images | Steam CDN images would be an extra network dependency. |
| Replay storage | replays are registered in place (path in DB), not copied | Parsed data lives in SQLite; the file is only needed for re-parsing. |
| Series step | 5 game seconds (`parser.SeriesStep`) | ~4k rows per match, enough for charts and death/lane positions. |
| Lane opponent | enemy with the smallest mean distance to me over 1:00–10:00 | Checked against OpenDota lanes on the test replay. Refine with roles in phase 3. |
| Release build | one ubuntu-24.04 job: native linux build + `GOOS=windows` cross-compile (no CGO on Windows) | Simpler and faster than a Windows runner. |
| Dev/test driving | `wails3 task build:server` + Playwright against the HTTP server | GUI can be checked headlessly without a display. |
| Parser v2 data model | purchases, casts, hits, wards, runes, stacks, buybacks as rows in `events` (new `key` column); damage/heal as per-player aggregates in `combat` (player, kind, other side, inflictor); cumulative farm gold and damage in `series` | Raw DAMAGE entries (~33k/match) are only needed aggregated; per-event rows stay for things with a time. |
| Ability hit heuristic | parser stores ability/item `Hit`s on enemy (non-illusion) heroes — DAMAGE with that inflictor or MODIFIER_ADD whose `modifier_ability` is it — merged within 0.2 s; a cast is a hit if one follows within `hit_window_sec` (setting, applied at view time). Hit rate is shown only for abilities that touched an enemy hero at least once in the match | Window stays tunable without re-parsing; buffs/blinks don't show a misleading 0%. |
| Idle periods | alive stretches ≥ `idle_threshold` with no LH/DN, neutral gold, damage dealt or taken (series step 5 s) | Computable from stored series, threshold tunable without re-parsing. |
| Lane & map zones | lane = lane band holding ≥40% of alive samples 1:00–10:00 (else jungle/roam); zones: own base, own lane, own half, enemy half (river along x+y=0) — coarse world-coordinate geometry | Matches OpenDota lanes for all 10 players of the test replay. |
| Ward kills | ward entity `m_lifeState` change matched to the ward DEATH combat log entry (±1 s); an entry whose attacker is the ward itself is expiry | Entity deletion comes ~7 s after death; matches OpenDota observer/sentry kills exactly. |
| Worst habits | last 10 filtered matches vs all earlier filtered matches (≥5), worse by ≥10%; 16 metrics with direction | Simple and explainable. |
| Weekday / hour | local time of game start (end − duration) | |
| Item "key" | UI marks items ≥1800 gold (OpenDota cost); consumables hidden by default | No reliable "core item" list in constants. |
| Download target | configured replay folder if it exists, else `<data dir>/replays`; existing `<id>.dem` is reused | Downloaded replays get picked up by the normal scan too. |
| Replay download | OpenDota `/matches/{id}` → `cluster` + `replay_salt` → `replay{cluster}.valve.net/570/{id}_{salt}.dem.bz2`. **The file is currently zstd despite the `.bz2` name** — detect by magic bytes, support both. 404 → "Valve already deleted this replay". | found during spike |

## Phases

### Phase 0 — bootstrap
- [x] `CLAUDE.md`, `PLAN.md`, `.gitignore` (testdata ignored)
- [x] GitHub repo `1rowvy/drstats` (public) + push

### Phase 1 — spike (`cmd/dump`)
- [x] `cmd/dump <file.dem>` → JSON: match info & players, every combat log type with samples,
      hero entity classes and their fields (position, gold, LH, level, …), game rules states
- [x] tick → game time function anchored at `GAME_IN_PROGRESS` (`parser.Clock`, pause-aware)
- [x] panic-safe parse wrapper (`parser.Safe`)
- [x] test on `testdata/` replay (skip if absent)
- [x] report: `docs/spike.md`

### Phase 2 — MVP
- [x] Wails v3 app skeleton (React+TS+Vite), dark theme
- [x] `internal/store`: SQLite, migrations, matches/players/series/events tables, `parser_version`, status (`pending/parsing/done/failed` + error)
- [x] Settings: Steam ID (account id or steam64), replay folder (OS defaults), parallelism (default 2), heuristic thresholds, map background file
- [x] Replay sources: folder scan on start + button, manual import (dialog + drag-n-drop)
- [x] Background parsing worker pool, progress via Wails events, failed matches keep error text
- [x] Domain extraction v1: match meta, players, my player detection, per-N-second time series (gold, xp, net worth, LH/DN, position), deaths, kills
- [x] Match list: hero, result, KDA, duration, date, mode, parse status
- [x] Match page tabs: Overview, Farm (NW/GPM/XPM charts: me / team avg / lane opponent), Deaths (list + points on map)
- [x] Map: world coords from cell+vec, schematic map (lanes, river) or user background
- [x] Constants (hero/item/ability names) from OpenDota, cached
- [x] Auto-update: updater wired to GitHub Releases + release workflow (win/linux)
- [x] README: build on Windows & Fedora, dev mode

### Phase 3 — everything else
- [x] Laning: LH/DN @5/@10, NW/XP diff vs lane opponent (detected by early positions), lane deaths
- [x] Gold sources: lane creeps / neutrals / heroes / buildings / passive; idle periods (threshold)
- [x] Items: purchase timings; key item timing vs my average on this hero
- [x] Deaths: killer, killing blow source, gold lost, buyback
- [x] Fights: KP%, damage dealt/taken by hero and source (autoattack/ability/item), healing, building damage
- [x] Abilities: casts per ability & active item, **heuristic** hit rate (damage/debuff within N s), labeled as heuristic
- [x] Vision & map: wards placed/dewarded, my wards killed, stacks, runes, movement heatmap, time share (own lane / jungle / enemy half / base)
- [x] Aggregates: winrate by hero / weekday / hour; per-hero averages & trend over last N; "worst habits" (last 10 worse than my avg); death heatmap per hero; filters (hero, period, W/L, mode)
- [x] Hero page, Dashboard
- [x] Download by match ID via OpenDota
- [x] "Re-parse all" button (parser_version)
