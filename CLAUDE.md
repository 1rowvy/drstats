# drstats — working notes for Claude

Personal desktop app that parses Dota 2 replays (`.dem`) and shows detailed stats
for one player (the owner). Single user, fully local: no servers, accounts or cloud
(the only network calls are OpenDota constants/replay download and the GitHub
Releases update check).

The full task spec lives in `PLAN.md` (phases, checklist, decisions). Keep it up to date:
tick items as they land and record any decision that deviates from the spec.

## Communication & workflow rules
- Talk to the owner in **Russian**. Code comments, commit messages, README, PR text — **English**.
- **Commit after every step** (one logical change per commit, conventional-ish prefix:
  `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`, `ci:`).
- **At the end of each phase: push to `origin main` yourself**, then stop and report
  to the owner what was done (and, after phase 1, what data a replay does / does not contain).
- Git identity comes from the owner's global git config — do not override it.
- Never commit replays: `testdata/` is gitignored.

## Stack
- **Wails v3** (stable since v3.0.0, 2026-10-26; pinned to v3.1.x). Linux uses GTK4 + WebKitGTK 6.0.
- **Go 1.26+**, `github.com/dotabuff/manta` for replay parsing.
- **SQLite via `modernc.org/sqlite`** (pure Go, no CGO → trivial Windows cross-compile and CI).
- Frontend: React + TypeScript + Vite, ECharts for charts, `<canvas>` for heatmaps/map.
- Auto-update: Wails v3 built-in updater with the GitHub Releases provider; releases
  are built by GitHub Actions on tag `v*` for windows/amd64 and linux/amd64.

## Layout
```
cmd/dump/          spike CLI: .dem -> JSON dump of raw data (match, players, combat log, entities)
internal/parser/   manta -> domain structs. NO imports of store/UI/Wails.
internal/store/    SQLite, embedded migrations
internal/stats/    aggregates over stored matches
internal/library/  folder scan, import, background parse worker pool
internal/settings/ settings model and defaults
internal/constants/ OpenDota constants cache
app.go / main.go   Wails services & bindings
frontend/          React app
testdata/          local replays (gitignored); tests skip when absent
```

## Rules for the parser
- Time: always convert ticks via the game-time function anchored at the tick where
  game rules state becomes `GAME_IN_PROGRESS` (game time 00:00). Pre-game is negative.
- manta can break after Dota patches: parsing must **never** crash the app. Recover
  panics, mark the match `failed`, store the error text, show it in the UI.
- Bump `parser.Version` whenever extraction logic changes (enables "re-parse all").
- Tests use replays from `testdata/`; if the file is missing, `t.Skip`, never fail.

## Commands
```
go test ./...                         # unit tests (replay tests skip without testdata)
go run ./cmd/dump testdata/X.dem > out.json
wails3 dev                            # dev mode with hot reload
wails3 build                          # production build for current OS (VERSION=x.y.z to stamp it)
wails3 build GOOS=windows             # cross-compile Windows from Linux (no CGO)
wails3 task build:server              # GUI-less HTTP build, for driving the UI with Playwright
```
