package settings

import "testing"

func TestParseAccountID(t *testing.T) {
	cases := map[string]uint32{
		"":                  0,
		"914467708":         914467708,
		"76561198874733436": 914467708,
		" https://steamcommunity.com/profiles/76561198874733436/ ": 914467708,
		"https://www.dotabuff.com/players/914467708":               914467708,
	}
	for in, want := range cases {
		got, err := ParseAccountID(in)
		if err != nil || got != want {
			t.Errorf("ParseAccountID(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"abc", "-1", "0", "76561197960265728"} {
		if _, err := ParseAccountID(bad); err == nil {
			t.Errorf("ParseAccountID(%q): want error", bad)
		}
	}
}

func TestNormalize(t *testing.T) {
	s := Settings{Parallelism: 0, HitWindowSec: -1}
	s.Normalize()
	if s.Parallelism != 1 || s.HitWindowSec != 1 || s.IdleThreshold != 20 {
		t.Errorf("%+v", s)
	}
}
