// Package constants fetches hero/item/ability names from OpenDota's constants
// API and caches the raw JSON in the store.
package constants

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1rowvy/drstats/internal/store"
)

// Sets fetched from OpenDota.
var Sets = []string{"heroes", "items", "abilities"}

// BaseURL of the OpenDota constants endpoint.
var BaseURL = "https://api.opendota.com/api/constants/"

// Hero is a hero's display info.
type Hero struct {
	ID   int    `json:"id"`
	Name string `json:"name"` // localized, e.g. "Anti-Mage"
	Attr string `json:"attr"` // str / agi / int / all
}

// Item is an item's display info.
type Item struct {
	Name string `json:"name"`
	Cost int    `json:"cost"`
}

// Names is the compact dictionary sent to the frontend.
type Names struct {
	Heroes    map[string]Hero   `json:"heroes"`     // by npc name (npc_dota_hero_*)
	HeroByID  map[int]string    `json:"hero_by_id"` // id -> npc name
	Items     map[string]Item   `json:"items"`      // by key without "item_" prefix
	Abilities map[string]string `json:"abilities"`  // by ability key -> display name
	FetchedAt int64             `json:"fetched_at"` // unix, oldest of the sets; 0 if any missing
	// Cooldowns of abilities and active items (items as "item_<key>"), for
	// the fight analysis; not sent to the frontend.
	Cooldowns map[string]Cooldown `json:"-"`
}

// Cooldown of an ability or item, seconds.
type Cooldown struct {
	Max    float64 // longest (lowest level)
	Levels int     // number of cooldown values (3 = ultimate-like)
	Item   bool
}

// Service loads constants from the cache and refreshes them from OpenDota.
type Service struct {
	st     *store.Store
	client *http.Client

	mu    sync.Mutex
	names *Names
}

// New creates the service.
func New(st *store.Store) *Service {
	return &Service{st: st, client: &http.Client{Timeout: 60 * time.Second}}
}

// Names returns the decoded dictionary (from cache; empty maps if never fetched).
func (s *Service) Names() (*Names, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.names != nil {
		return s.names, nil
	}
	n, err := s.decode()
	if err != nil {
		return nil, err
	}
	s.names = n
	return n, nil
}

// Missing reports whether any constants set has never been fetched.
func (s *Service) Missing() bool {
	for _, set := range Sets {
		if b, _, _ := s.st.Constant(set); b == nil {
			return true
		}
	}
	return false
}

// Refresh downloads every set and replaces the cache. Sets that fail keep
// their previous cached value.
func (s *Service) Refresh(ctx context.Context) error {
	var errs []error
	for _, set := range Sets {
		b, err := s.fetch(ctx, set)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", set, err))
			continue
		}
		if err := s.st.PutConstant(set, b); err != nil {
			errs = append(errs, err)
		}
	}
	s.mu.Lock()
	s.names = nil
	s.mu.Unlock()
	return errors.Join(errs...)
}

func (s *Service) fetch(ctx context.Context, set string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+set, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "drstats")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if !json.Valid(b) {
		return nil, errors.New("response is not JSON")
	}
	return b, nil
}

func (s *Service) decode() (*Names, error) {
	n := &Names{Heroes: map[string]Hero{}, HeroByID: map[int]string{}, Items: map[string]Item{}, Abilities: map[string]string{},
		Cooldowns: map[string]Cooldown{}}
	var oldest time.Time
	missing := false
	load := func(set string, v any) error {
		b, at, err := s.st.Constant(set)
		if err != nil {
			return err
		}
		if b == nil {
			missing = true
			return nil
		}
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
		return json.Unmarshal(b, v)
	}

	var heroes map[string]struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		LocalizedName string `json:"localized_name"`
		PrimaryAttr   string `json:"primary_attr"`
	}
	if err := load("heroes", &heroes); err != nil {
		return nil, fmt.Errorf("heroes: %w", err)
	}
	for _, h := range heroes {
		n.Heroes[h.Name] = Hero{ID: h.ID, Name: h.LocalizedName, Attr: h.PrimaryAttr}
		n.HeroByID[h.ID] = h.Name
	}

	var items map[string]struct {
		DName string          `json:"dname"`
		Cost  int             `json:"cost"`
		CD    json.RawMessage `json:"cd"`
	}
	if err := load("items", &items); err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}
	for k, it := range items {
		n.Items[k] = Item{Name: it.DName, Cost: it.Cost}
		if mx, lv := parseCD(it.CD); mx > 0 {
			n.Cooldowns["item_"+k] = Cooldown{Max: mx, Levels: lv, Item: true}
		}
	}

	var abilities map[string]struct {
		DName string          `json:"dname"`
		CD    json.RawMessage `json:"cd"`
	}
	if err := load("abilities", &abilities); err != nil {
		return nil, fmt.Errorf("abilities: %w", err)
	}
	for k, a := range abilities {
		if a.DName != "" {
			n.Abilities[k] = a.DName
		}
		if mx, lv := parseCD(a.CD); mx > 0 {
			n.Cooldowns[k] = Cooldown{Max: mx, Levels: lv}
		}
	}
	if !missing {
		n.FetchedAt = oldest.Unix()
	}
	return n, nil
}

// DecodeCooldowns reads the cooldowns from raw OpenDota "abilities" and
// "items" sets (either may be nil).
func DecodeCooldowns(abilities, items []byte) (map[string]Cooldown, error) {
	out := map[string]Cooldown{}
	var raw map[string]struct {
		CD json.RawMessage `json:"cd"`
	}
	if abilities != nil {
		if err := json.Unmarshal(abilities, &raw); err != nil {
			return nil, err
		}
		for k, a := range raw {
			if mx, lv := parseCD(a.CD); mx > 0 {
				out[k] = Cooldown{Max: mx, Levels: lv}
			}
		}
	}
	if items != nil {
		raw = nil
		if err := json.Unmarshal(items, &raw); err != nil {
			return nil, err
		}
		for k, it := range raw {
			if mx, lv := parseCD(it.CD); mx > 0 {
				out["item_"+k] = Cooldown{Max: mx, Levels: lv, Item: true}
			}
		}
	}
	return out, nil
}

// parseCD reads an OpenDota "cd" value: a number, a string ("120" or
// "50 40 30") or an array of either. Returns the longest value and the
// number of values; 0 when absent.
func parseCD(raw json.RawMessage) (float64, int) {
	if len(raw) == 0 {
		return 0, 0
	}
	var vals []string
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		for _, a := range arr {
			vals = append(vals, strings.Trim(string(a), `"`))
		}
	} else {
		vals = strings.Fields(strings.Trim(string(raw), `"`))
	}
	var mx float64
	n := 0
	for _, v := range vals {
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			continue
		}
		n++
		mx = math.Max(mx, f)
	}
	return mx, n
}

// HeroDisplay returns the localized hero name, falling back to a prettified
// npc name when constants are not available.
func (n *Names) HeroDisplay(npc string) string {
	if h, ok := n.Heroes[npc]; ok && h.Name != "" {
		return h.Name
	}
	return Prettify(strings.TrimPrefix(npc, "npc_dota_hero_"))
}

// Prettify turns "dragon_knight" into "Dragon Knight".
func Prettify(key string) string {
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
