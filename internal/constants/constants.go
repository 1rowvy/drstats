// Package constants fetches hero/item/ability names from OpenDota's constants
// API and caches the raw JSON in the store.
package constants

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	n := &Names{Heroes: map[string]Hero{}, HeroByID: map[int]string{}, Items: map[string]Item{}, Abilities: map[string]string{}}
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
		DName string `json:"dname"`
		Cost  int    `json:"cost"`
	}
	if err := load("items", &items); err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}
	for k, it := range items {
		n.Items[k] = Item{Name: it.DName, Cost: it.Cost}
	}

	var abilities map[string]struct {
		DName string `json:"dname"`
	}
	if err := load("abilities", &abilities); err != nil {
		return nil, fmt.Errorf("abilities: %w", err)
	}
	for k, a := range abilities {
		if a.DName != "" {
			n.Abilities[k] = a.DName
		}
	}
	if !missing {
		n.FetchedAt = oldest.Unix()
	}
	return n, nil
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
