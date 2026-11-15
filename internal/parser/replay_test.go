package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dotabuff/manta"
)

// testReplay returns a replay from testdata/ or skips the test when none is present
// (replays are not committed: they weigh tens of megabytes).
func testReplay(t *testing.T) string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "*.dem"))
	if len(files) == 0 {
		t.Skip("no replay in testdata/, skipping")
	}
	return files[0]
}

// TestClockOnReplay checks the clock against a real replay: the horn must be
// found, pre-game must last 90s, and game time must be monotonic.
func TestClockOnReplay(t *testing.T) {
	path := testReplay(t)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var clock Clock
	var preGameTick, postGameTick uint32
	last := -1
	err = Safe(func() error {
		p, err := manta.NewStreamParser(f)
		if err != nil {
			return err
		}
		p.OnEntity(func(e *manta.Entity, _ manta.EntityOp) error {
			if e.GetClassName() != "CDOTAGamerulesProxy" {
				return nil
			}
			clock.SetPaused(p.Tick, Int(e, "m_pGameRules.m_bGamePaused") == 1)
			st := Int(e, "m_pGameRules.m_nGameState")
			if st == last {
				return nil
			}
			last = st
			switch st {
			case StatePreGame:
				preGameTick = p.Tick
			case StateGameInProgress:
				clock.MarkStart(p.Tick)
			case StatePostGame:
				postGameTick = p.Tick
			}
			return nil
		})
		err = p.Start()
		clock.Close(p.Tick)
		return err
	})
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if !clock.Started() {
		t.Fatal("GAME_IN_PROGRESS never reached")
	}
	if got := clock.GameTime(clock.StartTick()); got != 0 {
		t.Errorf("game time at horn = %v, want 0", got)
	}
	if got := clock.GameTime(preGameTick); got < -91 || got > -89 {
		t.Errorf("game time at PRE_GAME = %v, want about -90", got)
	}
	if postGameTick == 0 || clock.GameTime(postGameTick) < 60 {
		t.Errorf("POST_GAME at %v, want a real game length", clock.GameTime(postGameTick))
	}
	t.Logf("start tick %d, duration %.0fs, pauses %v", clock.StartTick(), clock.GameTime(postGameTick), clock.Pauses())
}

func TestSafeRecoversPanic(t *testing.T) {
	err := Safe(func() error { panic("boom") })
	pe, ok := err.(*ParseError)
	if !ok || !pe.Panic || pe.Msg != "boom" {
		t.Fatalf("got %#v", err)
	}
}
