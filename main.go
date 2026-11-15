// drstats is a personal desktop app that parses Dota 2 replays and shows
// detailed stats for one player.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Version is overridden at build time: -ldflags "-X main.Version=1.2.3".
var Version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "drstats",
		Description: "Dota 2 replay stats",
		Services: []application.Service{
			application.NewService(&AppService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "drstats",
		Width:            1400,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        640,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(15, 17, 21),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// AppService exposes app-level info to the frontend.
type AppService struct{}

// Version returns the running build version ("dev" for local builds).
func (a *AppService) Version() string { return Version }
