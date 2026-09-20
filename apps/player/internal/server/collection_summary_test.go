package server

import (
	"bytes"
	"encoding/json"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"html/template"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestCollectionCardEscapesMetadataNameAsOneRouteSegment(t *testing.T) {
	t.Parallel()
	name := "28 Days/Weeks/Years Later Collection"
	homeHTML := homeTemplateSource()
	start := strings.Index(homeHTML, `<a class="curation-card" href="{{.Path}}">`)
	if start < 0 {
		t.Fatal("collection card template is missing")
	}
	end := strings.Index(homeHTML[start:], `</a>`) + start + len(`</a>`)
	view := template.Must(template.New("card").Funcs(httpguard.CSRFParseFuncs(uiIcon)).Parse(homeHTML[start:end]))
	var output bytes.Buffer
	if err := view.Execute(&output, catalog.CollectionSummary{Name: name}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `href="/collection/`+url.PathEscape(name)+`"`) {
		t.Fatalf("card = %s", output.String())
	}
}

func TestCollectionSummariesSeparateDefaultAndCustomSources(t *testing.T) { //nolint:cyclop // One compact projection check covers source, count, and JSON parity.
	t.Parallel()
	store := newListStore("")
	items := []library.Item{
		{ID: "default-item", Title: "First", Collection: "Default Set"},
		{ID: "custom-item", Title: "Second", ShowArtwork: "show-poster"},
	}
	if err := store.createCollection(t.Context(), "Custom Set"); err != nil {
		t.Fatal(err)
	}
	if err := store.setCollection(t.Context(), "Custom Set", items[1], true); err != nil {
		t.Fatal(err)
	}
	if err := store.setCollection(t.Context(), "Default Set", items[0], false); err != nil {
		t.Fatal(err)
	}

	sources, counts := map[string]string{}, map[string]int{}
	for _, summary := range store.collectionSummaries(items, "") {
		sources[summary.Name], counts[summary.Name] = summary.Source, summary.ItemCount
		if summary.Name == "Custom Set" && (len(summary.ArtworkIDs) != 1 || summary.ArtworkIDs[0] != "custom-item") {
			t.Fatalf("show artwork = %#v", summary.ArtworkIDs)
		}
	}
	if sources["Custom Set"] != "custom" || sources["Default Set"] != "default" || counts["Custom Set"] != 1 || counts["Default Set"] != 0 {
		t.Fatalf("sources = %#v, counts = %#v", sources, counts)
	}
	encoded, err := json.Marshal(store.collectionSummaries(items, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(encoded) || !bytes.Contains(encoded, []byte(`"source":"default"`)) || !bytes.Contains(encoded, []byte(`"source":"custom"`)) || !bytes.Contains(encoded, []byte(`"artworkIds":["custom-item"]`)) {
		t.Fatalf("invalid API projection: %q", encoded)
	}
}
