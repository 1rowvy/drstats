package heroimg

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestMiddlewareCachesAndPassesThrough(t *testing.T) {
	var hits atomic.Int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/antimage.png" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("\x89PNG\r\n\x1a\nfake"))
	}))
	defer cdn.Close()
	old := CDN
	CDN = cdn.URL + "/"
	defer func() { CDN = old }()

	c := New(t.TempDir())
	h := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }))

	get := func(path string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Code
	}
	if code := get("/index.html"); code != http.StatusTeapot {
		t.Fatalf("pass-through: got %d", code)
	}
	for range 2 {
		if code := get(Prefix + "antimage.png"); code != http.StatusOK {
			t.Fatalf("hero: got %d", code)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 CDN hit (cached), got %d", hits.Load())
	}
	if code := get(Prefix + "nope.png"); code != http.StatusNotFound {
		t.Fatalf("unknown hero: got %d", code)
	}
	get(Prefix + "nope.png")
	if hits.Load() != 2 {
		t.Fatalf("failed fetch must not be retried right away, hits=%d", hits.Load())
	}
	if code := get(Prefix + "..%2Fx.png"); code != http.StatusNotFound {
		t.Fatalf("bad name: got %d", code)
	}
}
