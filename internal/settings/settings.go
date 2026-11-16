// Package settings holds user settings and their defaults.
package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/1rowvy/drstats/internal/parser"
)

// Settings are the user-editable options. Stored as one JSON document.
type Settings struct {
	// SteamID as typed by the user: account id (32-bit) or steam64.
	SteamID string `json:"steam_id"`
	// ReplayDir is scanned for *.dem on start and on demand.
	ReplayDir string `json:"replay_dir"`
	// Parallelism is the number of replays parsed at once.
	Parallelism int `json:"parallelism"`
	// Heuristics thresholds (used by later analyses).
	HitWindowSec  float64 `json:"hit_window_sec"` // ability "hit": damage/debuff within N s of the cast
	IdleThreshold float64 `json:"idle_threshold"` // seconds without farm/fight considered idle
	// LH10Goal is the personal last-hits-at-10:00 target shown on the match and list pages.
	LH10Goal int `json:"lh10_goal"`
	// OpenDotaOff disables fetching per-match extras (positions, ranks,
	// benchmarks) from OpenDota. Off by default = fetching enabled.
	OpenDotaOff bool `json:"opendota_off"`
	// WatchOff stops watching the replay folder for new files (parsed as
	// soon as they appear). Off by default = watching.
	WatchOff bool `json:"watch_off"`
	// MapImage is an optional user-supplied minimap background (png/jpg).
	MapImage string `json:"map_image"`
	// AutoDownload fetches the replays of recent games (OpenDota history)
	// that have no replay yet. Off by default.
	AutoDownload bool `json:"auto_download"`
	// ReplayKeep is what happens to auto-downloaded replays after parsing:
	// KeepCap (default), KeepDelete or KeepAll.
	ReplayKeep string `json:"replay_keep"`
	// ReplayCapGB is the storage cap for auto-downloaded replays (KeepCap).
	ReplayCapGB float64 `json:"replay_cap_gb"`
	// TrayOff makes closing the window quit the app instead of hiding it to
	// the tray. Off by default = closing hides to the tray.
	TrayOff bool `json:"tray_off"`
	// NotifyOff silences the desktop notification shown when a fresh game of
	// the owner has been parsed. Off by default = notifications on.
	NotifyOff bool `json:"notify_off"`
}

// ReplayKeep values.
const (
	KeepCap    = "cap"    // keep up to ReplayCapGB, oldest removed first
	KeepDelete = "delete" // remove right after parsing
	KeepAll    = "all"    // never remove
)

// DefaultCapGB is the default storage cap for auto-downloaded replays.
const DefaultCapGB = 5

// CapBytes is ReplayCapGB in bytes.
func (s Settings) CapBytes() int64 { return int64(s.ReplayCapGB * (1 << 30)) }

// Defaults returns the settings used on first start.
func Defaults() Settings {
	return Settings{
		ReplayDir:     DefaultReplayDir(),
		Parallelism:   2,
		HitWindowSec:  1.0,
		IdleThreshold: 20,
		LH10Goal:      70,
		ReplayKeep:    KeepCap,
		ReplayCapGB:   DefaultCapGB,
	}
}

// Normalize fixes out-of-range values in place.
func (s *Settings) Normalize() {
	s.SteamID = strings.TrimSpace(s.SteamID)
	s.ReplayDir = strings.TrimSpace(s.ReplayDir)
	s.MapImage = strings.TrimSpace(s.MapImage)
	if s.Parallelism < 1 {
		s.Parallelism = 1
	}
	if n := runtime.NumCPU(); s.Parallelism > n {
		s.Parallelism = n
	}
	if s.HitWindowSec <= 0 {
		s.HitWindowSec = 1
	}
	if s.IdleThreshold <= 0 {
		s.IdleThreshold = 20
	}
	if s.LH10Goal <= 0 {
		s.LH10Goal = 70
	}
	switch s.ReplayKeep {
	case KeepCap, KeepDelete, KeepAll:
	default:
		s.ReplayKeep = KeepCap
	}
	if s.ReplayCapGB <= 0 {
		s.ReplayCapGB = DefaultCapGB
	}
	if s.ReplayCapGB > 1000 {
		s.ReplayCapGB = 1000
	}
}

// AccountID parses SteamID into a 32-bit account id. Empty input gives 0, nil.
func (s Settings) AccountID() (uint32, error) {
	return ParseAccountID(s.SteamID)
}

// ParseAccountID accepts an account id, a steam64 id, or a profile URL ending in one.
func ParseAccountID(in string) (uint32, error) {
	in = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(in), "/"))
	if in == "" {
		return 0, nil
	}
	if i := strings.LastIndex(in, "/"); i >= 0 {
		in = in[i+1:]
	}
	v, err := strconv.ParseUint(in, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Steam ID must be a number (account id or steam64), got %q", in)
	}
	if v > parser.Steam64Base {
		v -= parser.Steam64Base
	}
	if v == 0 || v > 0xFFFFFFFF {
		return 0, fmt.Errorf("Steam ID %q is out of range", in)
	}
	return uint32(v), nil
}

// DefaultReplayDir returns the Dota 2 replay folder of a standard Steam install,
// the first candidate that exists (or the first candidate if none does).
func DefaultReplayDir() string {
	const rel = "steamapps/common/dota 2 beta/game/dota/replays"
	var cands []string
	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if p := os.Getenv(env); p != "" {
				cands = append(cands, filepath.Join(p, "Steam", filepath.FromSlash(rel)))
			}
		}
		cands = append(cands, filepath.Join(`C:\Program Files (x86)\Steam`, filepath.FromSlash(rel)))
	default:
		home, _ := os.UserHomeDir()
		cands = append(cands,
			filepath.Join(home, ".steam/steam", rel),
			filepath.Join(home, ".local/share/Steam", rel),
			filepath.Join(home, ".var/app/com.valvesoftware.Steam/.local/share/Steam", rel),
		)
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return cands[0]
}
