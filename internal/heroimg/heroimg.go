// Package heroimg serves hero portraits to the frontend. Images are fetched
// from the Steam CDN on first use and cached on disk, so after the first view
// they work offline. Mounted as asset-server middleware under Prefix.
package heroimg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Prefix is the URL path the frontend requests: Prefix + "<short name>.png",
// e.g. /heroimg/antimage.png for npc_dota_hero_antimage, or
// Prefix + "render/<short name>.png" for the full-body render.
const Prefix = "/heroimg/"

// RenderDir is the sub-path (and cache subfolder) of the full-body renders.
const RenderDir = "render"

// CDN is the base of Valve's hero portraits (256x144 PNG).
var CDN = "https://cdn.cloudflare.steamstatic.com/apps/dota2/images/dota_react/heroes/"

// RenderCDN is the base of Valve's full-body hero renders (1440x1440 PNG, transparent).
var RenderCDN = "https://cdn.cloudflare.steamstatic.com/apps/dota2/videos/dota_react/heroes/renders/"

// retryAfter is how long a failed fetch is not retried (offline, unknown hero).
const retryAfter = 10 * time.Minute

var validName = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// Cache fetches and stores hero portraits in dir.
type Cache struct {
	dir    string
	client *http.Client

	mu     sync.Mutex
	failed map[string]time.Time
	locks  map[string]*sync.Mutex
}

// New creates a cache in dir (created on first write).
func New(dir string) *Cache {
	return &Cache{
		dir:    dir,
		client: &http.Client{Timeout: 20 * time.Second},
		failed: map[string]time.Time{},
		locks:  map[string]*sync.Mutex{},
	}
}

// Middleware serves Prefix paths and passes everything else on.
func (c *Cache) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, Prefix) {
			next.ServeHTTP(w, r)
			return
		}
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, Prefix), ".png")
		kind := ""
		if rest, ok := strings.CutPrefix(name, RenderDir+"/"); ok {
			kind, name = RenderDir, rest
		}
		path, err := c.get(r.Context(), kind, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=604800")
		http.ServeFile(w, r, path)
	})
}

// Get returns the local file of a hero portrait by short name ("antimage"),
// downloading it if needed.
func (c *Cache) Get(ctx context.Context, name string) (string, error) {
	return c.get(ctx, "", name)
}

// get returns a portrait (kind "") or a full-body render (kind RenderDir).
func (c *Cache) get(ctx context.Context, kind, name string) (string, error) {
	if !validName.MatchString(name) {
		return "", errors.New("bad hero name")
	}
	url := CDN + name + ".png"
	path := filepath.Join(c.dir, name+".png")
	if kind == RenderDir {
		url = RenderCDN + name + ".png"
		path = filepath.Join(c.dir, RenderDir, name+".png")
	}
	key := kind + "/" + name
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	c.mu.Lock()
	if t, ok := c.failed[key]; ok && time.Since(t) < retryAfter {
		c.mu.Unlock()
		return "", errors.New("recently failed")
	}
	l := c.locks[key]
	if l == nil {
		l = &sync.Mutex{}
		c.locks[key] = l
	}
	c.mu.Unlock()

	// One download per hero even when many badges request it at once.
	l.Lock()
	defer l.Unlock()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := c.fetch(ctx, url, path); err != nil {
		slog.Warn("hero image", "hero", key, "err", err)
		c.mu.Lock()
		c.failed[key] = time.Now()
		c.mu.Unlock()
		return "", err
	}
	return path, nil
}

func (c *Cache) fetch(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "drstats")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if len(b) < 8 || string(b[1:4]) != "PNG" {
		return errors.New("response is not a PNG")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
