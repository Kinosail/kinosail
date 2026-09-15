package metadata

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCastEnrichmentAndRecordProjection(t *testing.T) {
	fetch := func(_ context.Context, endpoint string, target any) error {
		if endpoint != "https://example.com/tv/42/credits" {
			t.Fatalf("endpoint = %s", endpoint)
		}
		return json.Unmarshal([]byte(`{"cast":[{"name":" Actor ","character":" Lead ","profile_path":"/face.jpg"},{"name":"Voice"}]}`), target)
	}
	result, err := enrichTMDBCast(t.Context(), "https://example.com", "tv", 42, fetch, func(id string) string { return "/cache/" + id + ".jpg" }, Result{Record: Record{Title: "Show"}})
	if err != nil || len(result.Record.ShowCast) != 2 || len(result.Images) != 1 || !result.Images[0].Person {
		t.Fatalf("result = %#v, %v", result, err)
	}
	items := ApplyRecords([]library.Item{{ID: "episode"}}, map[string]Record{"episode": result.Record}, func(string) string { return "" })
	if items[0].ShowCast[0].Name != "Actor" || items[0].ShowCast[0].Role != "Lead" || items[0].ShowCast[0].Image != "" {
		t.Fatalf("cast = %#v", items[0].ShowCast)
	}
	if result.Record.ShowCast[0].Image == "" {
		t.Fatal("projection mutated stored cast")
	}
}

func TestCastRejectsInvalidProviderBeforeArtwork(t *testing.T) {
	for _, body := range []string{
		`{"cast":[{"name":"Actor","profile_path":"/../secret"}]}`,
		`{"cast":[{"name":"Actor\u0000"}]}`,
		`{"cast":[{"name":"` + strings.Repeat("x", 201) + `"}]}`,
		`{"cast":[` + strings.Repeat(`{"name":"A"},`, 1000) + `{"name":"A"}]}`,
	} {
		calls := 0
		result, err := enrichTMDBCast(t.Context(), "https://example.com", "tv", 1, func(_ context.Context, _ string, target any) error { return json.Unmarshal([]byte(body), target) }, func(string) string { calls++; return "art" }, Result{Record: Record{Title: "Show"}})
		if err == nil || calls != 0 || len(result.Images) != 0 {
			t.Fatalf("invalid cast produced side effects: %#v %v calls=%d", result, err, calls)
		}
	}
}

func TestStoredCastBoundsAndCopy(t *testing.T) {
	for _, cast := range [][]library.Person{{{Name: ""}}, {{Name: "A", Role: strings.Repeat("x", 201)}}, {{Name: "A", Image: strings.Repeat("x", 4097)}}, make([]library.Person, 16)} {
		if Bounded(Record{Cast: cast}) || Bounded(Record{ShowCast: cast}) {
			t.Fatal("invalid persisted cast accepted")
		}
	}
	original := Record{ShowCast: []library.Person{{Name: "Original"}}}
	copied := Clone(original)
	copied.ShowCast[0].Name = "Changed"
	if original.ShowCast[0].Name != "Original" {
		t.Fatal("clone shares cast storage")
	}
}
