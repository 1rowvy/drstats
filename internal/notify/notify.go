// Package notify builds the post-game desktop notification: result, grade,
// the main reason the game went the way it did and the weekly plan goals of
// that game. Pure text, no UI imports.
package notify

import (
	"fmt"
	"math"
	"strings"

	"github.com/1rowvy/drstats/internal/stats"
)

// Fresh is how recent a game must be (its end, against now) to be announced:
// a bulk import or a re-parse of old replays stays silent.
const Fresh = 3 * 60 * 60 // seconds

// metric labels and formats, mirrored from frontend/src/gameMetrics.ts.
var metrics = map[string]struct {
	label string
	fmt   func(float64) string
}{
	"lh10":        {"добивания к 10:00", round},
	"deaths":      {"смерти", round},
	"deaths10":    {"смерти до 10:00", round},
	"avoid":       {"глупые смерти", round},
	"kp":          {"участие в убийствах", pct},
	"lane_nw":     {"нетворс к оппоненту на 10:00", signed},
	"mid_idle":    {"простой в мидгейме", pct},
	"farm_gpm":    {"фарм в минуту", round},
	"gpm":         {"GPM", round},
	"xpm":         {"XPM", round},
	"hero_dmg_pm": {"урон героям в минуту", round},
}

func round(v float64) string { return fmt.Sprint(int(math.Round(v))) }
func pct(v float64) string   { return fmt.Sprintf("%d%%", int(math.Round(v*100))) }
func signed(v float64) string {
	s := round(v)
	if v > 0 {
		s = "+" + s
	}
	return s
}

func metric(key string, v float64) (string, string) {
	if m, ok := metrics[key]; ok {
		return m.label, m.fmt(v)
	}
	return key, fmt.Sprintf("%.1f", v)
}

// Message is a ready notification.
type Message struct {
	Title string
	Body  string
}

// Build makes the notification for the focused player of v (must be "me").
// hero is the localized hero name; plan may have no plan (Plan == nil).
func Build(v *stats.MatchView, plan stats.PlanView, hero string) Message {
	var me *stats.PlayerView
	for i := range v.Players {
		if v.Players[i].PlayerID == v.Focus {
			me = &v.Players[i]
		}
	}
	if me == nil {
		return Message{}
	}
	win := me.Team == v.Meta.Winner
	title := "Поражение"
	if win {
		title = "Победа"
	}
	title += " · " + hero
	if v.Grade != nil {
		title += " · оценка " + v.Grade.Letter
	}
	min := int(v.Meta.Duration) / 60
	lines := []string{fmt.Sprintf("%d/%d/%d · %d:%02d · %d GPM", me.Kills, me.Deaths, me.Assists, min, int(v.Meta.Duration)%60, me.GPM)}

	if r := v.Reasons; r != nil && len(r.Main) > 0 {
		m := r.Main[0]
		label, val := metric(m.Key, m.Value)
		_, ref := metric(m.Key, m.Ref)
		head := "Проиграно на"
		if win {
			head = "Победу принесли"
		}
		lines = append(lines, fmt.Sprintf("%s: %s — %s (обычно %s)", head, label, val, ref))
	}

	if plan.Plan != nil && !plan.Expired {
		met, total := 0, 0
		var missed []string
		for _, g := range plan.Goals {
			for _, gg := range g.Games {
				if gg.MatchID != v.Meta.MatchID {
					continue
				}
				total++
				if gg.Met {
					met++
				} else {
					label, _ := metric(g.Key, 0)
					missed = append(missed, label)
				}
			}
		}
		if total > 0 {
			s := fmt.Sprintf("План: %d из %d целей", met, total)
			if len(missed) > 0 {
				s += " · мимо: " + strings.Join(missed, ", ")
			}
			lines = append(lines, s)
		}
	}
	return Message{Title: title, Body: strings.Join(lines, "\n")}
}
