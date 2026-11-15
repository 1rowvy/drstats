package opendota

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchHistory(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/42/matches" {
			http.NotFound(w, r)
			return
		}
		query = r.URL.RawQuery
		w.Write([]byte(`[
			{"match_id":10,"start_time":1000,"duration":1800,"game_mode":22,"lobby_type":7,"radiant_win":false,"player_slot":130,"hero_id":53,
			 "kills":5,"deaths":2,"assists":7,"last_hits":200,"denies":10,"gold_per_min":600,"xp_per_min":700,
			 "hero_damage":null,"tower_damage":3000,"lane":1,"lane_role":3,"is_roaming":null,"party_size":2},
			{"match_id":11,"hero_id":0,"radiant_win":true,"player_slot":0},
			{"match_id":12,"hero_id":5,"radiant_win":null,"player_slot":0}
		]`))
	}))
	defer srv.Close()
	old := PlayersURL
	PlayersURL = srv.URL + "/"
	defer func() { PlayersURL = old }()

	h, err := New().FetchHistory(context.Background(), 42, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 1 {
		t.Fatalf("got %d matches, want 1 (bad rows dropped)", len(h))
	}
	m := h[0]
	if m.Radiant() || !m.Win() || m.HeroDamage != 0 || m.TowerDamage != 3000 || m.GoldPerMin != 600 || m.PartySize != 2 || m.Lane != 1 {
		t.Fatalf("match = %+v", m)
	}
	if want := "date=3"; !contains(query, want) || !contains(query, "significant=0") || !contains(query, "project=gold_per_min") {
		t.Fatalf("query %q", query)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
