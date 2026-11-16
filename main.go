// drstats is a personal desktop app that parses Dota 2 replays and shows
// detailed stats for one player.
package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/1rowvy/drstats/internal/heroimg"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// Version is overridden at build time: -ldflags "-X main.Version=1.2.3".
var Version = "dev"

// Repository publishing releases for the auto-updater.
const releaseRepo = "1rowvy/drstats"

// AppID identifies the app: GTK application id / Wayland app_id (the Linux
// .desktop file is named after it) and the single-instance lock.
const AppID = "io.github.drstats"

// autostartFlag is passed by the login autostart entry: start minimised.
const autostartFlag = "--autostart"

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

func main() {
	svc := &App{notifier: newNotifier()}
	dir, err := dataDir()
	if err != nil {
		log.Fatal(err)
	}
	heroes := heroimg.New(filepath.Join(dir, "cache", "heroes"))
	var win *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "drstats",
		Description: "Dota 2 replay stats",
		Services:    []application.Service{application.NewService(svc), application.NewService(svc.notifier)},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: heroes.Middleware,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{ApplicationID: AppID},
		// Launching drstats again (or autostart while it already runs) brings
		// the open window to the front instead of a second copy.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: AppID,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if win != nil {
					win.UnMinimise()
					win.Show()
					win.Focus()
				}
			},
		},
	})

	// Autostart goes straight to the tray (or minimised when the tray is off).
	autostart := slices.Contains(os.Args[1:], autostartFlag)
	opts := application.WebviewWindowOptions{
		Title: "drstats",
		// The title bar is drawn by the frontend (drag area + window buttons).
		Frameless:        true,
		MinWidth:         minWidth,
		MinHeight:        minHeight,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(15, 17, 21),
		URL:              "/",
		StartState:       application.WindowStateNormal,
	}
	keeper := newWindowKeeper(dir)
	keeper.apply(&opts)
	if autostart {
		opts.Hidden = true // settings are not loaded yet: decided on start
	}
	win = app.Window.NewWithOptions(opts)
	setupTray(app, win, keeper, func() bool { return svc.settings().TrayOff }, trayIcon)
	setupNotifications(app, svc.notifier, win)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		ensureOnScreen(app, win)
		switch {
		case autostart && svc.settings().TrayOff:
			win.Show()
			win.Minimise()
		case !autostart:
			keeper.restoreMaximised(win)
		}
	})
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		files := e.Context().DroppedFiles()
		go svc.ImportPaths(files)
	})

	setupUpdater(app)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// setupUpdater wires the built-in updater to GitHub Releases. Dev builds skip it.
// On start it checks silently and opens the update window only if a newer
// release exists; afterwards it re-checks every few hours.
func setupUpdater(app *application.App) {
	if Version == "dev" {
		return
	}
	gh, err := github.New(github.Config{Repository: releaseRepo, ChecksumAsset: "SHA256SUMS"})
	if err != nil {
		slog.Error("updater provider", "err", err)
		return
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: Version,
		Providers:      []updater.Provider{gh},
		CheckInterval:  6 * time.Hour,
	}); err != nil {
		slog.Error("updater init", "err", err)
		return
	}
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			rel, err := app.Updater.Check(ctx)
			if err != nil {
				slog.Warn("update check", "err", err)
				return
			}
			if rel != nil {
				_ = app.Updater.CheckAndInstall(context.Background())
			}
		}()
	})
}
