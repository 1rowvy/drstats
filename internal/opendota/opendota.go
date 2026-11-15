// Package opendota fetches per-match extras that a replay does not contain
// (estimated positions, ranks, parties, benchmarks) from the OpenDota API.
// The replay stays the primary source; these only enrich it.
package opendota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// BaseURL is the OpenDota match endpoint (replaceable in tests).
var BaseURL = "https://api.opendota.com/api/matches/"

// Match fetch outcomes.
const (
	StatusOK       = "ok"       // full data stored
	StatusUnparsed = "unparsed" // OpenDota knows the match but has not parsed it yet: retry later
	StatusNotFound = "notfound" // OpenDota does not know the match
	StatusError    = "error"    // network / API error: retry later
)

// ErrNotFound means OpenDota returned 404 for the match.
var ErrNotFound = errors.New("OpenDota does not know this match")

// Bench is one OpenDota benchmark: the raw value and its percentile among
// recent games on the same hero (0..1).
type Bench struct {
	Raw float64 `json:"raw"`
	Pct float64 `json:"pct"`
}

// Player are the extras of one player.
type Player struct {
	HeroID     int              `json:"hero_id"`
	AccountID  uint32           `json:"account_id"`
	Position   int              `json:"position"`  // estimated position 1..5, 0 if unknown
	LaneRole   int              `json:"lane_role"` // 1 safe, 2 mid, 3 off, 4 jungle; 0 if unknown
	RankTier   int              `json:"rank_tier"` // e.g. 54 = Legend 4; 0 if unknown
	PartySize  int              `json:"party_size"`
	Benchmarks map[string]Bench `json:"benchmarks"`
}

// Match is what is kept from an OpenDota match.
type Match struct {
	MatchID uint64   `json:"match_id"`
	Parsed  bool     `json:"parsed"`
	Players []Player `json:"players"`
}

type apiMatch struct {
	MatchID uint64 `json:"match_id"`
	OdData  struct {
		HasParsed bool `json:"has_parsed"`
	} `json:"od_data"`
	Players []struct {
		HeroID      int              `json:"hero_id"`
		AccountID   *uint32          `json:"account_id"`
		PositionEst *int             `json:"position_est"`
		LaneRole    *int             `json:"lane_role"`
		RankTier    *int             `json:"rank_tier"`
		PartySize   *int             `json:"party_size"`
		Benchmarks  map[string]Bench `json:"benchmarks"`
	} `json:"players"`
}

// Decode converts an OpenDota /matches/{id} response.
func Decode(body []byte) (*Match, error) {
	var a apiMatch
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("decode OpenDota match: %w", err)
	}
	m := &Match{MatchID: a.MatchID, Parsed: a.OdData.HasParsed}
	deref := func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	}
	for _, p := range a.Players {
		pl := Player{HeroID: p.HeroID, Position: deref(p.PositionEst), LaneRole: deref(p.LaneRole),
			RankTier: deref(p.RankTier), PartySize: deref(p.PartySize), Benchmarks: p.Benchmarks}
		if p.AccountID != nil {
			pl.AccountID = *p.AccountID
		}
		if pl.Position < 0 || pl.Position > 5 {
			pl.Position = 0
		}
		if pl.Benchmarks == nil {
			pl.Benchmarks = map[string]Bench{}
		}
		m.Players = append(m.Players, pl)
	}
	return m, nil
}

// Client talks to OpenDota.
type Client struct {
	HTTP *http.Client
}

// New returns a client with a short timeout.
func New() *Client { return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}} }

// Fetch loads one match.
func (c *Client) Fetch(ctx context.Context, matchID uint64) (*Match, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+strconv.FormatUint(matchID, 10), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "drstats")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenDota: HTTP %d", resp.StatusCode)
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	return Decode(buf)
}
