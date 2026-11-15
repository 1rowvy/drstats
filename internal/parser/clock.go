package parser

import "sort"

// TickRate is the Source 2 server tick rate used by Dota 2 replays.
const TickRate = 30.0

// Game rules states (m_pGameRules.m_nGameState / DOTA_GameState).
const (
	StateInit             = 0
	StateWaitForPlayers   = 1
	StateHeroSelection    = 2
	StateStrategyTime     = 3
	StatePreGame          = 4
	StateGameInProgress   = 5
	StatePostGame         = 6
	StateDisconnect       = 7
	StateTeamShowcase     = 8
	StateCustomGameSetup  = 9
	StateWaitForMapToLoad = 10
	StateScenarioSetup    = 11
	StatePlayerDraft      = 12
)

// Clock converts replay ticks into in-game time, where 00:00 is the tick at
// which game rules enter GAME_IN_PROGRESS (the horn). Time spent in pauses is
// excluded; pre-horn time is negative (e.g. -90s at the start of pre-game).
//
// It is filled incrementally during the single parse pass (MarkStart,
// PauseStart/PauseEnd), and conversions are meant to be done afterwards,
// once the start tick is known.
type Clock struct {
	startTick uint32
	started   bool
	pauses    []pause // sorted, non-overlapping [from, to) tick ranges
	openPause *uint32
}

type pause struct{ from, to uint32 }

// MarkStart records the GAME_IN_PROGRESS tick. Only the first call counts.
func (c *Clock) MarkStart(tick uint32) {
	if !c.started {
		c.startTick, c.started = tick, true
	}
}

// Started reports whether the game reached GAME_IN_PROGRESS.
func (c *Clock) Started() bool { return c.started }

// StartTick returns the GAME_IN_PROGRESS tick (0 if not started).
func (c *Clock) StartTick() uint32 { return c.startTick }

// SetPaused feeds the current paused flag at the given tick; transitions are
// recorded as pause intervals. Repeated values are ignored.
func (c *Clock) SetPaused(tick uint32, paused bool) {
	switch {
	case paused && c.openPause == nil:
		t := tick
		c.openPause = &t
	case !paused && c.openPause != nil:
		if tick > *c.openPause {
			c.pauses = append(c.pauses, pause{*c.openPause, tick})
		}
		c.openPause = nil
	}
}

// Close ends a still-open pause at the given (last) tick.
func (c *Clock) Close(lastTick uint32) { c.SetPaused(lastTick, false) }

// Pauses returns the recorded pause intervals as [from,to) tick pairs.
func (c *Clock) Pauses() [][2]uint32 {
	out := make([][2]uint32, len(c.pauses))
	for i, p := range c.pauses {
		out[i] = [2]uint32{p.from, p.to}
	}
	return out
}

// pausedBetween returns the number of paused ticks within [a, b), a <= b.
func (c *Clock) pausedBetween(a, b uint32) uint32 {
	var n uint32
	i := sort.Search(len(c.pauses), func(i int) bool { return c.pauses[i].to > a })
	for ; i < len(c.pauses) && c.pauses[i].from < b; i++ {
		lo, hi := max(a, c.pauses[i].from), min(b, c.pauses[i].to)
		if hi > lo {
			n += hi - lo
		}
	}
	return n
}

// GameTime converts a tick into game time in seconds (negative before the horn).
// If the game never started, time is measured from tick 0.
func (c *Clock) GameTime(tick uint32) float64 {
	start := c.startTick
	if tick >= start {
		return float64(tick-start-c.pausedBetween(start, tick)) / TickRate
	}
	return -float64(start-tick-c.pausedBetween(tick, start)) / TickRate
}

// Tick converts game time back to the first tick at which that game time is
// reached (inverse of GameTime, pauses skipped).
func (c *Clock) Tick(gameTime float64) uint32 {
	// Walk forward from start, skipping pauses. Pauses are few, so linear is fine.
	if gameTime < 0 {
		need := uint32(-gameTime*TickRate + 0.5)
		t := c.startTick
		for t > 0 && need > 0 {
			t--
			if !c.isPaused(t) {
				need--
			}
		}
		return t
	}
	need := uint32(gameTime*TickRate + 0.5)
	t := c.startTick
	for _, p := range c.pauses {
		if p.to <= t {
			continue
		}
		if p.from >= t && p.from-t >= need {
			break
		}
		if p.from > t {
			need -= p.from - t
		}
		t = p.to
	}
	return t + need
}

func (c *Clock) isPaused(t uint32) bool {
	for _, p := range c.pauses {
		if t >= p.from && t < p.to {
			return true
		}
	}
	return false
}

// NewClock rebuilds a clock from a stored start tick and pause intervals.
func NewClock(startTick uint32, pauses [][2]uint32) *Clock {
	c := &Clock{startTick: startTick, started: true}
	for _, p := range pauses {
		c.pauses = append(c.pauses, pause{p[0], p[1]})
	}
	return c
}
