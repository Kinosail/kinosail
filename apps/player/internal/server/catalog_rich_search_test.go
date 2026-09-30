package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestCatalogAPIRichSearchPreservesCompleteMetadata(t *testing.T) {
	longPlot := strings.Repeat("Distant stars. ", 120) + "Café"
	for _, scenario := range []struct {
		name, query string
		item        library.Item
		match       bool
	}{
		{"ascii-title", "THE!!!signal", library.Item{Title: "The...Signal", Plot: longPlot}, true},
		{"accented-title", "CAFE", library.Item{Title: "Café", Plot: longPlot}, true},
		{"literal-plot", "quiet journey", library.Item{Title: "Signal", Plot: "A quiet journey. " + longPlot}, true},
		{"uppercase-plot", "quiet journey", library.Item{Title: "Signal", Plot: "A QUIET JOURNEY. " + longPlot}, true},
		{"title-show-boundary", "signal northern", library.Item{Title: "Signal", Show: "Northern Lights", Plot: longPlot}, true},
		{"title-plot-boundary", "signal quiet journey", library.Item{Title: "Signal", Plot: "quiet journey " + longPlot}, true},
		{"nonadjacent-fields", "signal harbor", library.Item{Title: "Signal", Plot: longPlot, Studio: "Harbor"}, false},
		{"cast-showcast-boundary", "captain morgan", library.Item{Plot: longPlot, Cast: []library.Person{{Name: "Sam Reed", Role: "Captain"}}, ShowCast: []library.Person{{Name: "Morgan Vale", Role: "Guide"}}}, true},
		{"ordinary-f-is-not-compatibility-f", "℉", library.Item{Plot: "F " + longPlot}, false},
		{"ordinary-a-is-not-compatibility-a", "𝐀", library.Item{Plot: "A " + longPlot}, false},
		{"compatibility-f", "℉", library.Item{Title: "℉", Plot: longPlot}, true},
		{"compatibility-a", "𝐀", library.Item{Title: "𝐀", Plot: longPlot}, true},
		{"plain-f-is-not-compatibility-f", "F", library.Item{Title: "℉", Plot: strings.Repeat("distant stars. ", 120)}, false},
		{"plain-a-is-not-compatibility-a", "A", library.Item{Title: "𝐀", Plot: strings.Repeat("moon. ", 120)}, false},
		{"beyond-prefix", "late needle", library.Item{Plot: longPlot + " late needle"}, true},
		{"cross-prefix", "late needle", library.Item{Plot: strings.Repeat("x", 509) + " late needle"}, true},
		{"split-unicode-prefix", "cafe needle", library.Item{Plot: strings.Repeat("x", 507) + " café needle"}, true},
		{"long-title-hit", "late needle", library.Item{Title: strings.Repeat("Long-title ", 100) + "late needle", Plot: longPlot}, true},
		{"long-title-boundary", "title tail", library.Item{Title: strings.Repeat("Long-title ", 100), Plot: "tail " + longPlot}, true},
		{"scratch-edge-title", "needle tail", library.Item{Title: strings.Repeat("x", 505) + "needle", Plot: "tail " + longPlot}, true},
		{"scratch-full-title", "needle tail", library.Item{Title: strings.Repeat("x", 506) + "needle", Plot: "tail " + longPlot}, true},
		{"scratch-edge-title-hit", "needle", library.Item{Title: strings.Repeat("x", 505) + "needle", Plot: longPlot}, true},
		{"scratch-full-title-hit", "needle", library.Item{Title: strings.Repeat("x", 506) + "needle", Plot: longPlot}, true},
		{"unicode-expansion-title-hit", "キロメートルキロメートル", library.Item{Title: strings.Repeat("㌖", 150), Plot: longPlot}, true},
		{"unicode-expansion-title-fallback", "late needle", library.Item{Title: strings.Repeat("㌖", 150), Plot: longPlot + " late needle"}, true},
		{"unicode-expansion-title-miss", "absent owl", library.Item{Title: strings.Repeat("㌖", 150), Plot: longPlot}, false},
		{"long-title-miss", "absent owl", library.Item{Title: strings.Repeat("Long-title ", 100), Plot: longPlot}, false},
		{"malformed-field-bytes", "az b c", library.Item{Title: "Az\x00b\x7fc", Plot: longPlot}, true},
		{"empty-title", "late needle", library.Item{Plot: longPlot + " late needle"}, true},
		{"repeated-spaces", "quiet journey", library.Item{Plot: "  quiet\t\njourney  " + longPlot}, true},
		{"absent", "absent owl", library.Item{Title: "Signal", Plot: longPlot}, false},
		{"punctuation-only", "!!!", library.Item{Title: "Signal", Plot: longPlot}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			assertCatalogRichSearch(t, scenario.item, scenario.query, scenario.match)
		})
	}
}

func assertCatalogRichSearch(t *testing.T, item library.Item, query string, match bool) {
	t.Helper()
	item.ID, item.Kind = "item", "video"
	index := memoryLibraryIndex([]library.Item{item}, true)
	progress, lists := newProgressStore(""), newListStore("")
	handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
		return browseLibrary(request, index, progress, lists)
	}, progress.ClientItem)
	response := httptest.NewRecorder()
	handler(response, ownerRequest("/api/v1/library?view=all&q="+url.QueryEscape(query)))
	var page struct {
		Total int `json:"total"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil {
		t.Fatalf("search response status=%d body=%s", response.Code, response.Body.String())
	}
	want := 0
	if match {
		want = 1
	}
	if page.Total != want || len(page.Items) != want || want == 1 && page.Items[0].ID != "item" {
		t.Fatalf("query %q returned total=%d items=%v; want %d", query, page.Total, page.Items, want)
	}
}
