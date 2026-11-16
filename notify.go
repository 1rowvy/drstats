package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/1rowvy/drstats/internal/notify"
)

// notifier wraps the Wails notification service so a missing notification
// daemon (no D-Bus session, failed toast registration) leaves the app running
// without notifications instead of failing to start. The service is kept in
// an unexported field so none of its methods become frontend bindings.
type notifier struct {
	svc *notifications.NotificationService
	ok  atomic.Bool
}

func newNotifier() *notifier { return &notifier{svc: notifications.New()} }

func (n *notifier) ServiceName() string { return "drstats.notifications" }

func (n *notifier) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	if err := n.svc.ServiceStartup(ctx, opts); err != nil {
		slog.Warn("notifications unavailable", "err", err)
		return nil
	}
	n.ok.Store(true)
	return nil
}

func (n *notifier) ServiceShutdown() error {
	if !n.ok.Load() {
		return nil
	}
	return n.svc.ServiceShutdown()
}

func (n *notifier) send(o notifications.NotificationOptions) error {
	if !n.ok.Load() {
		return fmt.Errorf("notifications unavailable")
	}
	return n.svc.SendNotification(o)
}

// announcedKey stores the match ids already announced, so a re-parse of a
// fresh game does not notify twice.
const (
	announcedKey  = "announced"
	announcedKeep = 50
)

// announce shows the post-game notification for a freshly parsed match of
// the owner. Old games (bulk imports, re-parses) and other people's replays
// stay silent.
func (a *App) announce(matchID int64) {
	if a.notifier == nil || !a.notifier.ok.Load() || a.settings().NotifyOff || a.accountID() == 0 {
		return
	}
	v, err := a.Match(matchID, -1)
	if err != nil || !v.IsMe || time.Now().Unix()-v.Meta.EndTime > notify.Fresh {
		return
	}
	a.announceMu.Lock()
	defer a.announceMu.Unlock()
	var seen []int64
	if _, err := a.st.GetSetting(announcedKey, &seen); err != nil {
		slog.Warn("load announced matches", "err", err)
	}
	if slices.Contains(seen, matchID) {
		return
	}
	plan, err := a.Plan()
	if err != nil {
		slog.Warn("plan for notification", "err", err)
	}
	var npc string
	for _, p := range v.Players {
		if p.PlayerID == v.Focus {
			npc = p.HeroName
		}
	}
	hero := strings.ReplaceAll(strings.TrimPrefix(npc, "npc_dota_hero_"), "_", " ")
	if n, err := a.names.Names(); err == nil {
		if h, ok := n.Heroes[npc]; ok && h.Name != "" {
			hero = h.Name
		}
	}
	m := notify.Build(v, plan, hero)
	if m.Title == "" {
		return
	}
	err = a.notifier.send(notifications.NotificationOptions{
		ID:    fmt.Sprintf("match-%d", matchID),
		Title: m.Title,
		Body:  m.Body,
		Data:  map[string]any{"match_id": strconv.FormatInt(matchID, 10)},
	})
	if err != nil {
		slog.Warn("post-game notification", "match", matchID, "err", err)
		return
	}
	seen = append(seen, matchID)
	if len(seen) > announcedKeep {
		seen = seen[len(seen)-announcedKeep:]
	}
	if err := a.st.SetSetting(announcedKey, seen); err != nil {
		slog.Warn("save announced matches", "err", err)
	}
}

// setupNotifications wires notification clicks: bring the window up and open
// the match.
func setupNotifications(app *application.App, n *notifier, win *application.WebviewWindow) {
	n.svc.OnNotificationResponse(func(r notifications.NotificationResult) {
		if r.Error != nil || r.Response.ActionIdentifier != notifications.DefaultActionIdentifier {
			return
		}
		id, err := strconv.ParseInt(fmt.Sprint(r.Response.UserInfo["match_id"]), 10, 64)
		if err != nil {
			return
		}
		application.InvokeAsync(func() {
			win.UnMinimise()
			win.Show()
			win.Focus()
		})
		app.Event.Emit(EventOpenMatch, id)
	})
}
