package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/1rowvy/drstats/internal/constants"
	"github.com/1rowvy/drstats/internal/download"
	"github.com/1rowvy/drstats/internal/library"
	"github.com/1rowvy/drstats/internal/settings"
	"github.com/1rowvy/drstats/internal/stats"
	"github.com/1rowvy/drstats/internal/store"
)

const settingsKey = "settings"

// Frontend events (besides the library ones).
const (
	EventConstants = "constants:updated" // payload: nil
	EventImported  = "library:imported"  // payload: ImportResult
	EventDownload  = "download:progress" // payload: download.Progress
)

func init() {
	application.RegisterEvent[library.ReplayEvent](library.EventReplay)
	application.RegisterEvent[library.Progress](library.EventProgress)
	application.RegisterEvent[library.ParseProgress](library.EventParsing)
	application.RegisterEvent[ImportResult](EventImported)
	application.RegisterEvent[application.Void](EventConstants)
	application.RegisterEvent[download.Progress](EventDownload)
}

// App is the single service bound to the frontend.
type App struct {
	app     *application.App
	dataDir string

	st    *store.Store
	lib   *library.Library
	names *constants.Service

	mu  sync.Mutex
	cfg settings.Settings
}

// ImportResult reports what an import / scan queued.
type ImportResult struct {
	Queued int    `json:"queued"`
	Error  string `json:"error"`
}

// ServiceStartup opens the database, loads settings, scans the replay folder
// and starts the parse workers.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.app = application.Get()
	dir, err := dataDir()
	if err != nil {
		return err
	}
	a.dataDir = dir
	a.st, err = store.Open(filepath.Join(dir, "drstats.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	a.cfg = settings.Defaults()
	if _, err := a.st.GetSetting(settingsKey, &a.cfg); err != nil {
		slog.Error("load settings", "err", err)
	}
	a.cfg.Normalize()

	a.names = constants.New(a.st)
	a.lib = library.New(a.st, func(name string, data any) { a.app.Event.Emit(name, data) })
	if err := a.lib.Start(a.cfg.Parallelism); err != nil {
		return err
	}
	go func() {
		n, err := a.lib.Scan(a.cfg.ReplayDir)
		if err != nil {
			slog.Warn("scan replay folder", "dir", a.cfg.ReplayDir, "err", err)
		}
		slog.Info("replay folder scanned", "dir", a.cfg.ReplayDir, "queued", n)
	}()
	if a.names.Missing() {
		go func() {
			if err := a.RefreshConstants(); err != nil {
				slog.Warn("fetch constants", "err", err)
			}
		}()
	}
	return nil
}

// ServiceShutdown stops the workers and closes the database.
func (a *App) ServiceShutdown() error {
	if a.lib != nil {
		a.lib.Stop()
	}
	if a.st != nil {
		return a.st.Close()
	}
	return nil
}

// dataDir is where the database lives: ~/.config/drstats or %AppData%\drstats.
func dataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "drstats")
	return dir, os.MkdirAll(dir, 0o755)
}

func (a *App) settings() settings.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) accountID() uint32 {
	id, _ := a.settings().AccountID()
	return id
}

// ---- app info ----

// AppInfo describes the running build.
type AppInfo struct {
	Version string `json:"version"`
	DataDir string `json:"data_dir"`
}

// Info returns the version and data directory.
func (a *App) Info() AppInfo { return AppInfo{Version: Version, DataDir: a.dataDir} }

// CheckForUpdates opens the updater window and runs the check/install flow.
func (a *App) CheckForUpdates() error {
	if Version == "dev" {
		return errors.New("updates are disabled in dev builds")
	}
	go func() {
		if err := a.app.Updater.CheckAndInstall(context.Background()); err != nil {
			slog.Warn("update flow", "err", err)
		}
	}()
	return nil
}

// ---- settings ----

// GetSettings returns the current settings.
func (a *App) GetSettings() settings.Settings { return a.settings() }

// SaveSettings validates and stores settings. A changed replay folder is scanned.
func (a *App) SaveSettings(s settings.Settings) (settings.Settings, error) {
	s.Normalize()
	if _, err := s.AccountID(); err != nil {
		return a.settings(), err
	}
	if err := a.st.SetSetting(settingsKey, s); err != nil {
		return a.settings(), err
	}
	a.mu.Lock()
	old := a.cfg
	a.cfg = s
	a.mu.Unlock()
	a.lib.SetParallelism(s.Parallelism)
	if s.ReplayDir != old.ReplayDir {
		go a.ScanReplayDir()
	}
	return s, nil
}

// DefaultReplayDir returns the OS default replay folder.
func (a *App) DefaultReplayDir() string { return settings.DefaultReplayDir() }

// PickDirectory shows a folder chooser; "" when cancelled.
func (a *App) PickDirectory(title string) (string, error) {
	return a.app.Dialog.OpenFile().SetTitle(title).
		CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(false).
		PromptForSingleSelection()
}

// PickMapImage shows an image chooser for the minimap background; "" when cancelled.
func (a *App) PickMapImage() (string, error) {
	return a.app.Dialog.OpenFile().SetTitle("Map background").
		AddFilter("Images", "*.png;*.jpg;*.jpeg;*.webp").
		PromptForSingleSelection()
}

// MapImage returns the configured map background as a data URL ("" if none or unreadable).
func (a *App) MapImage() string {
	path := a.settings().MapImage
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("read map image", "path", path, "err", err)
		return ""
	}
	typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if typ == "" {
		typ = "image/png"
	}
	return "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(b)
}

// ---- library ----

// ListMatches returns all replays with summaries for "me".
func (a *App) ListMatches() ([]store.MatchRow, error) {
	rows, err := a.st.ListMatches(a.accountID())
	if rows == nil {
		rows = []store.MatchRow{}
	}
	return rows, err
}

// Match returns the match page data; focus < 0 means "me".
func (a *App) Match(matchID int64, focus int) (*stats.MatchView, error) {
	cfg := a.settings()
	return stats.BuildMatchView(a.st, matchID, a.accountID(), focus, stats.Options{HitWindow: cfg.HitWindowSec, IdleThreshold: cfg.IdleThreshold})
}

// Dashboard returns aggregates over "my" matches under a filter (hero page:
// filter by hero).
func (a *App) Dashboard(f stats.Filter) (*stats.Dashboard, error) {
	return stats.BuildDashboard(a.st, a.accountID(), f)
}

// Progress returns the parse queue state.
func (a *App) Progress() library.Progress { return a.lib.Progress() }

// ScanReplayDir scans the configured replay folder.
func (a *App) ScanReplayDir() ImportResult {
	n, err := a.lib.Scan(a.settings().ReplayDir)
	return a.importResult(n, err)
}

// ImportDialog lets the user pick replay files and queues them.
func (a *App) ImportDialog() (ImportResult, error) {
	files, err := a.app.Dialog.OpenFile().SetTitle("Import replays").
		AddFilter("Dota 2 replays", "*.dem").
		PromptForMultipleSelection()
	if err != nil || len(files) == 0 {
		return ImportResult{}, err
	}
	return a.ImportPaths(files), nil
}

// ImportPaths queues files or folders (drag-n-drop).
func (a *App) ImportPaths(paths []string) ImportResult {
	n, err := a.lib.Import(paths)
	return a.importResult(n, err)
}

func (a *App) importResult(n int, err error) ImportResult {
	r := ImportResult{Queued: n}
	if err != nil {
		r.Error = err.Error()
	}
	a.app.Event.Emit(EventImported, r)
	return r
}

// ReparseAll queues parsed replays again; onlyStale limits it to replays
// parsed by an older parser version (and failed ones).
func (a *App) ReparseAll(onlyStale bool) (int, error) { return a.lib.ReparseAll(onlyStale) }

// DownloadMatch starts downloading a replay by match id (or match URL) in the
// background; progress and the result come as download:progress events, and
// the file is imported when done. It saves into the replay folder when it
// exists, else into the app data folder.
func (a *App) DownloadMatch(input string) (uint64, error) {
	id, err := download.ParseMatchID(input)
	if err != nil {
		return 0, err
	}
	dir := a.settings().ReplayDir
	if st, err := os.Stat(dir); dir == "" || err != nil || !st.IsDir() {
		dir = filepath.Join(a.dataDir, "replays")
	}
	go func() {
		c := download.New(func(p download.Progress) { a.app.Event.Emit(EventDownload, p) })
		path, err := c.Fetch(context.Background(), id, dir)
		if err != nil {
			slog.Warn("download replay", "match", id, "err", err)
			return
		}
		a.ImportPaths([]string{path})
	}()
	return id, nil
}

// Reparse queues a replay for parsing again.
func (a *App) Reparse(replayID int64) error { return a.lib.Reparse(replayID) }

// ---- constants ----

// Names returns hero/item/ability display names.
func (a *App) Names() (*constants.Names, error) { return a.names.Names() }

// RefreshConstants re-downloads constants from OpenDota.
func (a *App) RefreshConstants() error {
	err := a.names.Refresh(context.Background())
	a.app.Event.Emit(EventConstants)
	return err
}
