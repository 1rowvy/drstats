package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Default and minimum window size.
const (
	defaultWidth  = 1400
	defaultHeight = 900
	minWidth      = 1000
	minHeight     = 640
)

// windowState is the window geometry remembered between runs.
type windowState struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	HasPos    bool `json:"has_pos"`
	Maximised bool `json:"maximised"`
}

// windowKeeper loads, tracks and saves the window state (window.json in the data dir).
type windowKeeper struct {
	path string
	mu   sync.Mutex
	st   windowState
}

func newWindowKeeper(dir string) *windowKeeper {
	k := &windowKeeper{path: filepath.Join(dir, "window.json"), st: windowState{Width: defaultWidth, Height: defaultHeight}}
	if b, err := os.ReadFile(k.path); err == nil {
		var st windowState
		if json.Unmarshal(b, &st) == nil && st.Width >= minWidth && st.Height >= minHeight {
			k.st = st
		}
	}
	return k
}

// apply puts the remembered geometry into the window options.
func (k *windowKeeper) apply(o *application.WebviewWindowOptions) {
	k.mu.Lock()
	defer k.mu.Unlock()
	o.Width, o.Height = k.st.Width, k.st.Height
	if k.st.HasPos {
		o.InitialPosition = application.WindowXY
		o.X, o.Y = k.st.X, k.st.Y
	}
	if k.st.Maximised && o.StartState == application.WindowStateNormal {
		o.StartState = application.WindowStateMaximised
	}
}

// capture reads the current geometry of a visible window. A maximised or
// minimised window keeps the last normal size and position.
func (k *windowKeeper) capture(w *application.WebviewWindow) {
	if !w.IsVisible() || w.IsMinimised() {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.st.Maximised = w.IsMaximised()
	if k.st.Maximised || w.IsFullscreen() {
		return
	}
	if wd, ht := w.Size(); wd >= minWidth && ht >= minHeight {
		k.st.Width, k.st.Height = wd, ht
	}
	// Wayland does not report window positions: (0, 0) there means "unknown".
	if x, y := w.Position(); x != 0 || y != 0 {
		k.st.X, k.st.Y, k.st.HasPos = x, y, true
	}
}

func (k *windowKeeper) save() {
	k.mu.Lock()
	b, _ := json.MarshalIndent(k.st, "", "  ")
	k.mu.Unlock()
	if err := os.WriteFile(k.path, b, 0o644); err != nil {
		slog.Warn("save window state", "err", err)
	}
}

// ensureOnScreen recentres the window when the remembered position is off
// every screen (a monitor was unplugged).
func ensureOnScreen(app *application.App, w *application.WebviewWindow) {
	x, y := w.Position()
	wd, _ := w.Size()
	for _, s := range app.Screen.GetAll() {
		a := s.WorkArea
		if x+wd > a.X+50 && x < a.X+a.Width-50 && y >= a.Y-20 && y < a.Y+a.Height-50 {
			return
		}
	}
	if len(app.Screen.GetAll()) > 0 {
		w.Center()
	}
}

// setupTray adds the tray icon and makes closing the window hide it to the
// tray (unless trayOff says otherwise). quit really exits the app.
func setupTray(app *application.App, w *application.WebviewWindow, keeper *windowKeeper, trayOff func() bool, icon []byte) {
	var quitting atomic.Bool
	show := func() {
		w.UnMinimise()
		w.Show()
		w.Focus()
	}
	quit := func() {
		quitting.Store(true)
		keeper.capture(w)
		keeper.save()
		app.Quit()
	}
	menu := app.NewMenu()
	menu.Add("Открыть drstats").OnClick(func(*application.Context) { show() })
	menu.AddSeparator()
	menu.Add("Выход").OnClick(func(*application.Context) { quit() })

	tray := app.SystemTray.New()
	tray.SetIcon(icon)
	tray.SetTooltip("drstats")
	tray.SetMenu(menu)
	tray.OnClick(func() {
		if w.IsVisible() && !w.IsMinimised() {
			w.Hide()
		} else {
			show()
		}
	})

	// Track the geometry as it changes; it is written on close / quit / shutdown.
	for _, ev := range []events.WindowEventType{events.Common.WindowDidResize, events.Common.WindowDidMove,
		events.Common.WindowMaximise, events.Common.WindowUnMaximise} {
		w.OnWindowEvent(ev, func(*application.WindowEvent) { keeper.capture(w) })
	}
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		keeper.capture(w)
		keeper.save()
		if quitting.Load() || trayOff() {
			return
		}
		w.Hide()
		e.Cancel()
	})
	app.OnShutdown(keeper.save)
}
