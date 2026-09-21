package server

import (
	"bytes"
	"html/template"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/servertest"
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
	servertest.CollectionSources(t, store.createCollection, store.setCollection, store.collectionSummaries)
}
