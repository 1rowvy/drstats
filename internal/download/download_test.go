package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestParseMatchID(t *testing.T) {
	for in, want := range map[string]uint64{
		"9091456344": 9091456344, " https://www.opendota.com/matches/9091456344/ ": 9091456344,
		"https://www.dotabuff.com/matches/123": 123,
	} {
		if got, err := ParseMatchID(in); err != nil || got != want {
			t.Errorf("%q: %d %v", in, got, err)
		}
	}
	if _, err := ParseMatchID("abc"); err == nil {
		t.Error("garbage accepted")
	}
}

func TestFetch(t *testing.T) {
	demo := []byte("PBDEMS2\x00" + strings.Repeat("replay-bytes", 1000))
	var zbuf bytes.Buffer
	zw, _ := zstd.NewWriter(&zbuf)
	zw.Write(demo)
	zw.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/matches/1":
			fmt.Fprint(w, `{"match_id":1,"cluster":123,"replay_salt":456}`)
		case "/api/matches/2":
			fmt.Fprint(w, `{"match_id":2,"cluster":123,"replay_salt":789}`) // expired on Valve's side
		case "/api/matches/3":
			fmt.Fprint(w, `{"match_id":3}`)
		case "/570/1_456.dem.bz2":
			w.Write(zbuf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	OpenDotaURL = srv.URL + "/api/matches/"
	ReplayURL = func(cluster int, id uint64, salt int64) string { return fmt.Sprintf("%s/570/%d_%d.dem.bz2", srv.URL, id, salt) }

	var stages []string
	c := New(func(p Progress) { stages = append(stages, p.Stage) })
	dir := t.TempDir()
	path, err := c.Fetch(context.Background(), 1, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, demo) {
		t.Errorf("decompressed %d bytes, want %d", len(got), len(demo))
	}
	if stages[len(stages)-1] != "done" {
		t.Errorf("stages %v", stages)
	}
	if _, err := c.Fetch(context.Background(), 2, dir); !errors.Is(err, ErrExpired) {
		t.Errorf("expired: %v", err)
	}
	if _, err := c.Fetch(context.Background(), 3, dir); !errors.Is(err, ErrNoSalt) {
		t.Errorf("no salt: %v", err)
	}
	if _, err := c.Fetch(context.Background(), 4, dir); !errors.Is(err, ErrNotFound) {
		t.Errorf("not found: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}
