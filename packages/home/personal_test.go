package home

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type personalProbe struct {
	t       *testing.T
	request *http.Request
	items   []library.Item
	calls   []string
}

func (probe *personalProbe) active(request *http.Request, items []library.Item) []string {
	probe.calls = append(probe.calls, "active")
	if request != probe.request || &items[0] != &probe.items[0] {
		probe.t.Fatal("active did not receive canonical input")
	}
	return []string{"resume"}
}

func (probe *personalProbe) listed(request *http.Request, items []library.Item) []library.Item {
	probe.calls = append(probe.calls, "listed")
	if request != probe.request || &items[0] != &probe.items[0] {
		probe.t.Fatal("listed did not receive canonical input")
	}
	return items
}

func (probe *personalProbe) played(request *http.Request, items []library.Item) []library.Item {
	probe.calls = append(probe.calls, "played")
	if request != probe.request || &items[0] == &probe.items[0] {
		probe.t.Fatal("played did not receive an isolated input")
	}
	items[0].Title = "Played"
	return items
}

func (probe *personalProbe) collections(items []library.Item) []string {
	probe.calls = append(probe.calls, "collections")
	if &items[0] != &probe.items[0] {
		probe.t.Fatal("collections did not receive canonical input")
	}
	return []string{"one", "two"}
}

func (probe *personalProbe) playlists(request *http.Request) []string {
	probe.calls = append(probe.calls, "playlists")
	if request != probe.request {
		probe.t.Fatal("playlists request was replaced")
	}
	return []string{"one"}
}

func TestPersonalBuildsPlayerProjectionWithoutAliasingItems(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	items := []library.Item{{ID: "movie", Title: "Movie"}}
	probe := &personalProbe{t: t, request: request, items: items, calls: make([]string, 0, 5)}
	projection := Personal(request, items, probe.active, probe.listed, probe.played, probe.collections, probe.playlists)
	if !reflect.DeepEqual(probe.calls, []string{"listed", "active", "played", "collections", "playlists"}) {
		t.Fatalf("calls = %#v", probe.calls)
	}
	if projection.Continue[0] != "resume" || projection.List[0].ID != "movie" || projection.Played[0].Title != "Played" || projection.CollectionCount != 2 || projection.PlaylistCount != 1 {
		t.Fatalf("projection = %#v", projection)
	}
	if items[0].Title != "Movie" {
		t.Fatalf("source item was mutated: %#v", items[0])
	}
}
