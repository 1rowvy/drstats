package opendota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Profile is the part of an OpenDota player profile drstats uses.
type Profile struct {
	RankTier        int `json:"rank_tier"`        // e.g. 54 = Legend 4, 80 = Immortal; 0 if hidden / uncalibrated
	LeaderboardRank int `json:"leaderboard_rank"` // Immortal place; 0 if none
}

// DecodeProfile converts an OpenDota /players/{id} response.
func DecodeProfile(body []byte) (Profile, error) {
	var raw struct {
		RankTier        *int `json:"rank_tier"`
		LeaderboardRank *int `json:"leaderboard_rank"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Profile{}, fmt.Errorf("decode OpenDota profile: %w", err)
	}
	var p Profile
	if raw.RankTier != nil {
		p.RankTier = *raw.RankTier
	}
	if raw.LeaderboardRank != nil {
		p.LeaderboardRank = *raw.LeaderboardRank
	}
	return p, nil
}

// FetchProfile loads the account's current medal.
func (c *Client) FetchProfile(ctx context.Context, accountID uint32) (Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, PlayersURL+strconv.FormatUint(uint64(accountID), 10), nil)
	if err != nil {
		return Profile{}, err
	}
	req.Header.Set("User-Agent", "drstats")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Profile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("OpenDota: HTTP %d", resp.StatusCode)
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Profile{}, err
	}
	return DecodeProfile(buf)
}
