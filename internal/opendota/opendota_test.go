package opendota

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDecodeTestdata(t *testing.T) {
	b, err := os.ReadFile("../../testdata/9091456344.opendota.json")
	if err != nil {
		t.Skip("testdata missing")
	}
	m, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Parsed || len(m.Players) != 10 {
		t.Fatalf("parsed=%v players=%d", m.Parsed, len(m.Players))
	}
	p := m.Players[0]
	if p.HeroID != 83 || p.Position != 5 || p.RankTier != 44 || p.Benchmarks["gold_per_min"].Raw != 214 {
		t.Fatalf("player 0 = %+v", p)
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/1" {
			w.Write([]byte(`{"match_id":1,"od_data":{"has_parsed":false},"players":[{"hero_id":5,"account_id":null}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL + "/"
	defer func() { BaseURL = old }()

	m, err := New().Fetch(context.Background(), 1)
	if err != nil || m.Parsed || len(m.Players) != 1 || m.Players[0].HeroID != 5 || m.Players[0].Position != 0 {
		t.Fatalf("m=%+v err=%v", m, err)
	}
	if _, err := New().Fetch(context.Background(), 2); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
