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
| Hero icons | Valve portraits from the Steam CDN (`dota_react/heroes/<name>.png`), fetched on first use by asset-server middleware (`/heroimg/`, `internal/heroimg`) and cached in `<data dir>/cache/heroes`; initials tile as fallback | Owner asked for real icons (2026-11). Cache keeps it offline after the first view; failures retry after 10 min. |
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

| UI redesign (mockups 2026-11) | dark theme with orange accent, mono numerals; nav Matches / Heroes / Dashboard / Settings; import/scan/download moved to the Matches page (download appears when the search is an unknown match id) | Owner's mockups. |
| Role label | guessed from lane + LH@10: mid → Мид, safe lane ≥25 LH → Керри, off lane ≥18 LH → Оффлейн, otherwise Саппорт; jungle/roam | No role data in replays. |
| Death "no vision" | no allied observer alive within 1400 units of the death spot | Observer day vision radius; sentries ignored. |
| Map background | bundled 7.41 game minimap (`frontend/public/minimap-7.41.jpg`, from Liquipedia's game-asset copy), image covers world [-10400, 10300], calibrated on fountains/ancients/towers/Roshan of a 7.41 replay; user picture in settings still overrides; schematic only if loading fails | Owner asked for the real map (2026-11). Needs a refresh when a patch reshapes the map. |
| Map zones | `stats.Region`: base / river (top·mid·bot) / lane + side / jungle + side, coarse geometry; used for deaths, idle periods, hero death top-zones | Human-readable locations; no tower-level precision. |
| OpenDota match extras | after parsing, `/matches/{id}` is fetched in the background (1 req/1.1 s) and `position_est`, `rank_tier`, `party_size`, `benchmarks` are stored in `od_players` (keyed by match + hero, no FK so re-parse keeps them). Role = OpenDota position when known, else own lane heuristic. Unparsed / failed fetches retry after 6 h; switch-off in settings | Owner: "тянуть и оттуда и оттуда" — replay stays the primary source, OpenDota fills what a replay can't know. |
| UI redesign v2 (mockup PDF 2026-11-16) | top nav Матчи / Дашборд / Герои / Импорт, settings behind a gear; fonts Unbounded + Inter + JetBrains Mono bundled via fontsource (offline); match tabs Разбор / Фарм / Драки / Смерти / Способности + extra «Вижн» (kept so ward/rune data stays visible) | Owner's second mockup set. |
| Parser v3 | building / Roshan kills (`objective` events), hero-to-hero damage in 5 s buckets per attacker/target/inflictor (`hdmg` events, ~1–3k rows/match), pause intervals in `matches.pauses` | Needed for the game-flow timeline, fight breakdowns and exact `demo_gototick` ticks. |
| Fights | 5 s buckets with ≥600 hero damage or a hero death, merged across gaps ≤10 s; kept if a death happened or ≥4000 damage, and the focused player dealt/took ≥150 or got a kill/assist; won = more enemy deaths (tie: gold swing); gold swing = change of team NW difference over the fight | Simple, explainable; on the test replay every kill of every player falls into a detected fight. |
| Match grade | weighted percentile (KDA 1.5, GPM 1.2, XPM .8, hero dmg/min 1, KP 1, deaths 1 (lower better), LH@10 .8, tower dmg .5) against the player's other matches — same hero when ≥8 others, else all; ×85 + 15 for a win; letters A+…D; none with <5 reference matches | Personal, needs no external data. |
| Moments / verdict | generated on the client from fights, deaths, objectives, item timings, hit rates, idle time, OpenDota benchmarks | Texts are UI; backend gives structured data. |
| OpenDota match history | `/players/{account}/matches?significant=0` (projected basic fields: hero, result, K/D/A, LH/DN, GPM/XPM, hero/tower damage, healing, lane, party, mode) stored in `od_history`; full fetch first, then `date=<days since last sync + 1>`; synced on start (if older than 1 h), on Steam ID change and on demand; off with the OpenDota switch. Dashboard source «Все игры» merges it with replays (replay row wins for the same match); replay-only metrics (LH@10, wards, stacks, deaths map, fastest items, …) stay computed over replays only. Hero page stays on replays. Empty history → hint about «Expose Public Match Data» | Owner: dashboard over replays only is a biased sample (2026-11-16). |
| Auto-download | setting `auto_download` (off by default): matches of the OpenDota history from the last 15 days without a replay are downloaded one by one into `<data dir>/replays/auto` and parsed by the normal queue; outcomes in `auto_downloads` (notfound is final, expired is final only for matches older than 2 days — Valve answers 404 until a fresh replay is uploaded; no salt → OpenDota parse requested; failures retry after 1 h, matches younger than 2 days every 5 min for the first 12 attempts). Runs after each history sync and every 5 min | Owner (2026-11-16): a replay is ~50–100 MB, the parsed match ~1.5 MB. |
| Replay storage cap | `replay_keep`: `cap` (default, `replay_cap_gb` = 5) / `delete` (right after parsing) / `all`. Applies **only** to `<data dir>/replays/auto`, never to the Dota folder or manual downloads; only parsed replays are removed, oldest match first; `replays.pruned_at` marks them, re-parse skips them | Owner chose the cap. A pruned match keeps its data but cannot be re-parsed by a newer parser. |
| Installers | Windows: per-user NSIS (`%LOCALAPPDATA%\Programs\drstats`, no UAC, ru/en) so the updater can replace the exe; Linux: tarball with `install.sh` into `~/.local` (no rpm/deb: a root-owned binary would break self-update). Both named `*-installer.*`, which the updater's asset matcher skips; the updater archives keep exactly one top-level entry | Owner asked for an installer (2026-11-16). |
| Autostart | Wails `Autostart` (HKCU Run / XDG autostart), id `drstats`, flag `--autostart` → window starts hidden in the tray (minimised when the tray is off); single instance (`io.github.drstats`) shows the running window; GTK app id `io.github.drstats` matches the Linux .desktop file | Owner asked for a tray (2026-11-16): replaces the earlier "no tray, start minimised". |
| Tray & window state | Tray icon (app icon) with «Открыть drstats» / «Выход»; click toggles the window; closing the window hides it to the tray unless setting `tray_off`. Window size / position / maximised kept in `<data dir>/window.json`, captured on resize / move / close and saved on close and quit; position unknown on Wayland is not stored; off-screen position → centred | Owner (2026-11-16). |
| Folder watching | poll the replay folder every 20 s, files must be ≥15 s old (Dota may still be writing); setting `watch_off` | Mockup «Слежу за папкой / Парсить новые сразу»; polling is portable and cheap. |
| Import stages | the queue shows percent only, not header/entities/combat-log stages | Progress is measured by bytes read; manta has no stage hooks. |
| Per-replay parse progress | bytes read from the .dem, `library:parsing` event, ≥2% steps | Cheap and monotonic; no manta hook needed. |

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
- [x] Map: world coords from cell+vec, bundled game minimap (schematic fallback) or user background
- [x] Hero portraits instead of initials (Steam CDN, disk cache)
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

### UI redesign (owner mockups)
- [x] Theme, sidebar with icons and parse queue card
- [x] Matches: summary cards (winrate 20, KDA, LH@10 vs goal, GPM vs previous 20), search/hero/mode/result filters, per-replay parse progress, error details modal
- [x] Match overview: head stats, net worth vs opponent, laning card, gold sources, item timings vs average, ability hit rates, idle timeline; scoreboard below
- [x] Deaths tab: numbered map markers, movement layer, death cards with region, killers, killing blow, vision and buyback tags
- [x] Hero page: hero tabs, recent-vs-all cards, worst habits with explanations, GPM last 20, winrate by time of day, death zones, key item timings (avg / best / count)
- [x] Setting: LH@10 goal
- [x] OpenDota extras: estimated position (role), rank, party, benchmark percentiles on the match page; settings toggle and "fetch now" button

### UI redesign v2 (mockup PDF, 2026-11-16)
- [x] Parser v3: objectives, bucketed hero damage, pauses; parse time per replay
- [x] Stats: team advantage + ticks on the series axis, fights, match grades, dashboard extras (hours, activity, records, streak, party/solo, fastest items)
- [x] Replay folder watching (setting)
- [x] Theme, fonts, top nav; Matches cards with grades; Import page
- [x] Match: Разбор (flow, moments, verdict), Фарм, Драки, Смерти; Hero page; Dashboard

### Dashboard over full OpenDota history
- [x] OpenDota player history client + `od_history` table + background sync
- [x] Dashboard source «Все игры / Только реплеи», merged aggregates
- [x] Dashboard UI: source switch, sync status, OpenDota-only matches
- [x] Dashboard: last 10 games, tilt card (game-in-session winrate, after win / after loss, current streak; session = gap ≤ 1 h), habits (last 10 vs earlier) over all heroes
- [x] Lane analysis: match tab «Линия» (verdict at 10:00 = NW diff + XP diff / 2, ±500 → won / lost; head to head at 5:00 / 10:00, both sides of the lane, 0–10 min charts, lane deaths, usual result vs the opponent hero); lane stats card on dashboard and hero page (replays only)
- [x] Game flow on dashboard and hero page: phases lane 0–10 / mid 10–25 / late 25+ (GPM, XPM, LH/min, deaths per 10 min, KP, team lead change, winrate if the phase was won / lost), thrown / comeback games (team lead or deficit ≥ 5k; your deaths, KP and GPM while it swung), winrate by game length (all games)
- [x] Match «Разбор»: phases of the game against your usual numbers on the hero, thrown / comeback note (same computation as the dashboard)

### Auto-download of missing replays
- [x] Store: `auto_downloads`, `replays.pruned_at`, missing-replay query; re-parse skips pruned
- [x] Downloader worker + storage cap pruning, settings
- [x] UI: settings and state on the Import page, pruned mark in the queue

### Packaging
- [x] App icon
- [x] Start at login, single instance
- [x] Windows / Linux installers in releases
