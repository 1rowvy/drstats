# drstats

Personal desktop app that parses Dota 2 replays (`.dem`) and shows detailed stats
for one player. Fully local: replays are parsed on your machine and stored in a
SQLite database. The only network calls are OpenDota constants (hero/item names)
and the GitHub Releases update check.

Built with [Wails v3](https://v3.wails.io) (Go + WebView), React + TypeScript,
[manta](https://github.com/dotabuff/manta) and pure-Go SQLite.

## Using it

1. **Settings → Steam ID**: your account id (`914467708`), steam64
   (`76561198874733436`) or a profile URL. It is how the app finds "you" in a match.
2. **Replay folder**: defaults to the Steam install
   (`…/steamapps/common/dota 2 beta/game/dota/replays`). It is scanned on start
   and with the **Scan** button.
3. Replays elsewhere: **Import…** or drag `.dem` files / folders onto the window.

Replays are parsed in the background (2 at a time by default, ~1 s per replay).
A replay that fails to parse is kept with its error text and can be retried.

Data lives in `~/.config/drstats/drstats.db` (Linux) or
`%AppData%\drstats\drstats.db` (Windows).

## Building

Requirements: Go 1.26+, Node.js 22+, and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
wails3 doctor   # checks system dependencies
```

### Fedora

```sh
sudo dnf install gcc pkg-config gtk4-devel webkitgtk6.0-devel
wails3 build                       # -> bin/drstats
wails3 build GOOS=windows          # cross-compile -> bin/drstats.exe (no CGO needed)
```

### Windows

Install Go, Node.js and the Wails CLI (above). WebView2 is preinstalled on
Windows 10/11. Then in the repo:

```powershell
wails3 build                       # -> bin\drstats.exe
```

Production builds report version `dev` unless one is passed:
`wails3 build VERSION=1.2.3`. Dev builds never check for updates.

## Development

```sh
wails3 dev                         # app with hot reload (Go rebuilds, Vite HMR)
go test ./...                      # unit tests
go run ./cmd/dump testdata/X.dem > out.json   # raw replay dump (research spike)
```

Tests that need a real replay look for `testdata/*.dem` (gitignored) and skip
when there is none. If `testdata/<match_id>.opendota.json` (OpenDota's
`/api/matches/<id>` response) sits next to it, the parser is cross-checked
against it.

Layout:

```
cmd/dump/            replay -> JSON dump (spike)
internal/parser/     manta -> domain structs (no store/UI imports)
internal/store/      SQLite + embedded migrations
internal/library/    folder scan, import, background parse workers
internal/stats/      views/aggregates over stored matches
internal/settings/   settings model and defaults
internal/constants/  OpenDota constants cache
app.go, main.go      Wails service and app wiring
frontend/            React app (bindings in frontend/bindings are generated)
```

## Releases and auto-update

Pushing a tag `vX.Y.Z` runs `.github/workflows/release.yml`: it builds
`drstats-linux-amd64.tar.gz` and `drstats-windows-amd64.zip` with that version,
writes `SHA256SUMS` and publishes a GitHub Release. Installed apps check the
latest release on start and every 6 hours; when a newer one exists, an update
window offers to download, verify (SHA-256) and restart into it.

```sh
git tag v0.1.0 && git push origin v0.1.0
```
