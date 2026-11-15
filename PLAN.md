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
- [ ] Wails v3 app skeleton (React+TS+Vite), dark theme
- [ ] `internal/store`: SQLite, migrations, matches/players/series/events tables, `parser_version`, status (`pending/parsing/done/failed` + error)
- [ ] Settings: Steam ID (account id or steam64), replay folder (OS defaults), parallelism (default 2), heuristic thresholds, map background file
- [ ] Replay sources: folder scan on start + button, manual import (dialog + drag-n-drop)
- [ ] Background parsing worker pool, progress via Wails events, failed matches keep error text
- [ ] Domain extraction v1: match meta, players, my player detection, per-N-second time series (gold, xp, net worth, LH/DN, position), deaths, kills
- [ ] Match list: hero, result, KDA, duration, date, mode, parse status
- [ ] Match page tabs: Overview, Farm (NW/GPM/XPM charts: me / team avg / lane opponent), Deaths (list + points on map)
- [ ] Map: world coords from cell+vec, schematic map (lanes, river) or user background
- [ ] Constants (hero/item/ability names) from OpenDota, cached
- [ ] Auto-update: updater wired to GitHub Releases + release workflow (win/linux)
- [ ] README: build on Windows & Fedora, dev mode

### Phase 3 — everything else
- [ ] Laning: LH/DN @5/@10, NW/XP diff vs lane opponent (detected by early positions), lane deaths
- [ ] Gold sources: lane creeps / neutrals / heroes / buildings / passive; idle periods (threshold)
- [ ] Items: purchase timings; key item timing vs my average on this hero
- [ ] Deaths: killer, killing blow source, gold lost, buyback
- [ ] Fights: KP%, damage dealt/taken by hero and source (autoattack/ability/item), healing, building damage
- [ ] Abilities: casts per ability & active item, **heuristic** hit rate (damage/debuff within N s), labeled as heuristic
- [ ] Vision & map: wards placed/dewarded, my wards killed, stacks, runes, movement heatmap, time share (own lane / jungle / enemy half / base)
- [ ] Aggregates: winrate by hero / weekday / hour; per-hero averages & trend over last N; "worst habits" (last 10 worse than my avg); death heatmap per hero; filters (hero, period, W/L, mode)
- [ ] Hero page, Dashboard
- [ ] Download by match ID via OpenDota
- [ ] "Re-parse all" button (parser_version)
