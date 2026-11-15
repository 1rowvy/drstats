package constants

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/1rowvy/drstats/internal/store"
)

func TestRefreshAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/heroes":
			w.Write([]byte(`{"1":{"id":1,"name":"npc_dota_hero_antimage","localized_name":"Anti-Mage","primary_attr":"agi"}}`))
		case "/items":
			w.Write([]byte(`{"blink":{"dname":"Blink Dagger","cost":2250}}`))
		case "/abilities":
			w.Write([]byte(`{"antimage_mana_break":{"dname":"Mana Break"},"x":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL + "/"
	defer func() { BaseURL = old }()

	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New(st)
	if !s.Missing() {
		t.Fatal("fresh db must miss constants")
	}
	n, _ := s.Names()
	if n.FetchedAt != 0 || n.HeroDisplay("npc_dota_hero_dragon_knight") != "Dragon Knight" {
		t.Errorf("fallback: %+v", n)
	}
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	n, err = s.Names()
	if err != nil {
		t.Fatal(err)
	}
	if n.HeroDisplay("npc_dota_hero_antimage") != "Anti-Mage" || n.HeroByID[1] != "npc_dota_hero_antimage" ||
		n.Items["blink"].Cost != 2250 || n.Abilities["antimage_mana_break"] != "Mana Break" || n.FetchedAt == 0 {
		t.Errorf("decoded: %+v", n)
	}
	if _, ok := n.Abilities["x"]; ok {
		t.Error("abilities without a display name must be dropped")
	}
}
