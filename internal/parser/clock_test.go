package parser

import "testing"

func TestClockNoPauses(t *testing.T) {
	var c Clock
	c.MarkStart(6552)
	c.MarkStart(9999) // ignored
	cases := map[uint32]float64{6552: 0, 6582: 1, 3852: -90, 6552 + 30*600: 600}
	for tick, want := range cases {
		if got := c.GameTime(tick); got != want {
			t.Errorf("GameTime(%d) = %v, want %v", tick, got, want)
		}
		if got := c.Tick(want); got != tick {
			t.Errorf("Tick(%v) = %d, want %d", want, got, tick)
		}
	}
}

func TestClockWithPauses(t *testing.T) {
	var c Clock
	// pre-game pause of 10s, then horn, then a 60s pause at 1:00
	c.SetPaused(1000, true)
	c.SetPaused(1300, false)
	c.MarkStart(4000)
	c.SetPaused(4000+30*60, true)
	c.SetPaused(4000+30*60, true) // repeated, ignored
	c.SetPaused(4000+30*120, false)

	check := func(tick uint32, want float64) {
		t.Helper()
		if got := c.GameTime(tick); got != want {
			t.Errorf("GameTime(%d) = %v, want %v", tick, got, want)
		}
	}
	check(4000, 0)
	check(4000+30*60, 60)
	check(4000+30*90, 60)        // inside the pause: time stands still
	check(4000+30*120, 60)       // pause end
	check(4000+30*130, 70)       // 10s after resume
	check(4000-30*10, -10)       // before horn, no pause in between
	check(1270, -90)             // inside the pre-game pause: 2730 ticks minus 30 paused
	check(900, -(3100-300)/30.0) // before the pre-game pause: whole pause excluded

	if got := c.Tick(70); got != 4000+30*130 {
		t.Errorf("Tick(70) = %d, want %d", got, 4000+30*130)
	}
	if got := c.Tick(30); got != 4000+30*30 {
		t.Errorf("Tick(30) = %d, want %d", got, 4000+30*30)
	}
}

func TestClockOpenPauseClosed(t *testing.T) {
	var c Clock
	c.MarkStart(0)
	c.SetPaused(300, true)
	c.Close(600)
	if got := c.GameTime(900); got != 20 {
		t.Errorf("GameTime(900) = %v, want 20", got)
	}
	if len(c.Pauses()) != 1 {
		t.Fatalf("pauses = %v", c.Pauses())
	}
}
