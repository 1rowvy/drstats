// drstats is a personal desktop app that parses Dota 2 replays and shows
// detailed stats for one player.
package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// Version is overridden at build time: -ldflags "-X main.Version=1.2.3".
var Version = "dev"

// Repository publishing releases for the auto-updater.
const releaseRepo = "1rowvy/drstats"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	svc := &App{}
	app := application.New(application.Options{
		Name:        "drstats",
		Description: "Dota 2 replay stats",
		Services:    []application.Service{application.NewService(svc)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "drstats",
		Width:            1400,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        640,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(15, 17, 21),
		URL:              "/",
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
