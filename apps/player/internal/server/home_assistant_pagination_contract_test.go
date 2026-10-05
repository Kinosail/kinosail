package server_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalogapi"
)

type q12LibraryPage struct {
	Items []catalogapi.ClientItem
	View string
	Total, Offset, Limit int
}

func q12ReadPage(t *testing.T, fixture *q12PaginationFixture, token, route, query string, offset int) q12LibraryPage {
	t.Helper()
	status, body := fixture.get(t, token, q12PageTarget(route, query, offset, 200))
	if status != http.StatusOK { t.Fatal("Q12 page status is not successful") }
	var page q12LibraryPage
	if json.Unmarshal(body, &page) != nil { t.Fatal("Q12 page is not canonical JSON") }
	if page.Offset != offset || page.Limit != 200 || len(page.Items) > page.Limit || page.View != "all" {
		t.Fatal("Q12 public pagination metadata is inconsistent")
	}
	return page
}

func q12AssertExtent(t *testing.T, pages []q12LibraryPage, total int, counts []int) {
	t.Helper()
	if len(pages) != len(counts) { t.Fatal("Q12 extent fixture mismatch") }
	seen := make(map[string]bool)
	titles := []string{}
	for number, page := range pages {
		if page.Total != total || len(page.Items) != counts[number] { t.Fatal("Q12 displayed/total count mismatch") }
		for _, item := range page.Items {
			q12AppendCanonicalItem(t, item, seen)
			titles = append(titles, item.Title)
		}
	}
	if len(seen) != total || !slices.IsSorted(titles) { t.Fatal("Q12 complete ordered extent differs") }
}

func q12AppendCanonicalItem(t *testing.T, item catalogapi.ClientItem, seen map[string]bool) {
	t.Helper()
	if item.ID == "" || seen[item.ID] { t.Fatal("Q12 canonical identity is missing or repeated") }
	if item.Kind != "video" || item.Title == "" { t.Fatal("Q12 canonical item projection differs") }
	seen[item.ID] = true
}

func TestHomeAssistantPublicLibraryPaginationAndQuery(t *testing.T) {
	fixture := newQ12PaginationFixture(t)
	const route = "/api/v1/home-assistant/library"
	for _, token := range []string{fixture.owner, fixture.grant} {
		first := q12ReadPage(t, fixture, token, route, "", 0)
		last := q12ReadPage(t, fixture, token, route, "", 200)
		q12AssertExtent(t, []q12LibraryPage{first, last}, 208, []int{200, 8})
		searched := q12ReadPage(t, fixture, token, route, "Q12 Page", 0)
		final := q12ReadPage(t, fixture, token, route, "Q12 Page", 200)
		q12AssertExtent(t, []q12LibraryPage{searched, final}, 201, []int{200, 1})
		empty := q12ReadPage(t, fixture, token, route, "", 400)
		if empty.Total != 208 || len(empty.Items) != 0 { t.Fatal("Q12 empty final extent is not truthful") }
	}
	// Safe counts only. This does not describe adapter or HA frontend acceptance.
	t.Log("Q12 public control: Owner/scoped grant totals208/query201; pages200+8/200+1; empty0")
}

func TestHomeAssistantPublicLibraryRejectsInvalidPagination(t *testing.T) {
	fixture := newQ12PaginationFixture(t)
	for _, query := range []string{
		"limit=0", "limit=201", "offset=-1", "offset=1000001",
		"offset=0&offset=200", "limit=200&limit=1", "unexpected=1",
		"q="+strings.Repeat("x", 513), "q=a&q=b",
	} {
		status, body := fixture.get(t, fixture.grant, "/api/v1/home-assistant/library?"+query)
		if status != http.StatusBadRequest || !strings.Contains(string(body), `"error"`) ||
			strings.Contains(string(body), `"items"`) || strings.Contains(string(body), `"total"`) {
			t.Fatal("Q12 invalid pagination was accepted or disclosed Library data")
		}
	}
}

func TestHomeAssistantRejectsOrdinaryViewerAndLibraryKeepsVisibility(t *testing.T) {
	fixture := newQ12PaginationFixture(t)
	status, body := fixture.get(t, fixture.viewer, "/api/v1/home-assistant/library?limit=200")
	if status != http.StatusForbidden || strings.Contains(string(body), `"items"`) || strings.Contains(string(body), `"total"`) {
		t.Fatal("Q12 HA route disclosed data to an ordinary Viewer")
	}
	first := q12ReadPage(t, fixture, fixture.viewer, "/api/v1/library", "", 0)
	last := q12ReadPage(t, fixture, fixture.viewer, "/api/v1/library", "", 200)
	q12AssertExtent(t, []q12LibraryPage{first, last}, 201, []int{200, 1})
	for _, page := range []q12LibraryPage{first, last} {
		for _, item := range page.Items {
			if strings.Contains(item.Title, "Private") { t.Fatal("Q12 restricted item or total leaked") }
		}
	}
}
