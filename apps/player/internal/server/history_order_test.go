package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

type libraryHistoryPage struct {
	Items []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"items"`
	Sort    string            `json:"sort"`
	Total   int               `json:"total"`
	Offset  int               `json:"offset"`
	Limit   int               `json:"limit"`
	Letters []json.RawMessage `json:"letters"`
}

func TestLibraryHistoryViewListsMostRecentFirstBeyondOnePage(t *testing.T) {
	items := snapshotFixtureItems(600)
	fixture := newSnapshotFixture(t, items)
	owner := viewerProfile{ID: "owner", Owner: true}
	watched := make([]int, 0, 300)
	for position := range items {
		if items[position].Kind == "video" && items[position].Show == "" && len(watched) < cap(watched) {
			watched = append(watched, position)
		}
	}
	started := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	values := make(map[string]catalog.PlaybackState, len(watched)+1)
	for order, position := range watched {
		values[owner.ID+":"+items[position].ID] = catalog.PlaybackState{Seconds: 30, Updated: started.Add(time.Duration(order) * time.Minute)}
	}
	values["family:"+items[watched[0]].ID] = catalog.PlaybackState{Seconds: 5, Updated: started.Add(24 * time.Hour)}
	fixture.progress.Replace(values)
	recent := make([]string, 0, len(watched))
	for _, position := range slices.Backward(watched) {
		recent = append(recent, items[position].ID)
	}
	if items[watched[len(watched)-1]].Title <= items[watched[0]].Title {
		t.Fatal("fixture must make the newest history item sort last by title")
	}

	page := func(target string) libraryHistoryPage {
		t.Helper()
		response := httptest.NewRecorder()
		fixture.mux.ServeHTTP(response, snapshotRequest(t.Context(), http.MethodGet, target, owner))
		var body libraryHistoryPage
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK {
			t.Fatalf("%s = %d %.400q: %v", target, response.Code, response.Body.String(), err)
		}
		return body
	}
	ids := func(body libraryHistoryPage) []string {
		result := make([]string, 0, len(body.Items))
		for _, item := range body.Items {
			result = append(result, item.ID)
		}
		return result
	}
	for _, test := range []struct {
		target string
		want   []string
	}{
		{"/api/v1/library?view=history&sort=title&offset=0&limit=24", recent[:24]},
		{"/api/v1/library?view=history&limit=200", recent[:200]},
		{"/api/v1/library?view=history&sort=title&offset=200&limit=200", recent[200:]},
		{"/api/v1/library?q=&view=history&sort=title&offset=24&limit=24", recent[24:48]},
	} {
		body := page(test.target)
		if !slices.Equal(ids(body), test.want) || body.Total != len(recent) || body.Sort != "title" || body.Letters != nil {
			t.Fatalf("%s = total %d sort %q letters %d first %v, want first %v of %d without letters", test.target, body.Total, body.Sort, len(body.Letters), ids(body)[:min(3, len(body.Items))], test.want[:3], len(recent))
		}
	}

	history := httptest.NewRecorder()
	fixture.mux.ServeHTTP(history, snapshotRequest(t.Context(), http.MethodGet, "/api/v1/history?limit=24", owner))
	var direct historyPageBody
	if err := json.Unmarshal(history.Body.Bytes(), &direct); err != nil || !slices.Equal(historyIDs(direct), recent[:24]) {
		t.Fatalf("/api/v1/history order = %v, %v; want %v", historyIDs(direct), err, recent[:24])
	}

	added := page("/api/v1/library?view=history&sort=added&limit=3")
	if want := []string{items[watched[len(watched)-1]].ID, items[watched[len(watched)-2]].ID, items[watched[len(watched)-3]].ID}; !slices.Equal(ids(added), want) || added.Sort != "added" {
		t.Fatalf("added history = %v, want %v", ids(added), want)
	}

	rendered := httptest.NewRecorder()
	fixture.mux.ServeHTTP(rendered, snapshotRequest(t.Context(), http.MethodGet, "/?view=history", owner))
	body := rendered.Body.String()
	newest, oldest := strings.Index(body, items[watched[len(watched)-1]].Title), strings.Index(body, items[watched[len(watched)-100]].Title)
	if rendered.Code != http.StatusOK || newest < 0 || oldest < 0 || newest > oldest || strings.Contains(body, items[watched[0]].Title) || strings.Contains(body, "data-title-jump") {
		t.Fatalf("/?view=history = %d newest@%d older@%d, want newest first, one page, no title jump", rendered.Code, newest, oldest)
	}
	if !strings.Contains(body, `<option value="title" selected>Recently played</option>`) || strings.Contains(body, `<option value="title" selected>Title</option>`) {
		t.Fatalf("/?view=history sort menu does not label its default order as Recently played")
	}
	german := httptest.NewRecorder()
	fixture.mux.ServeHTTP(german, snapshotRequest(t.Context(), http.MethodGet, "/?view=history&lang=de", owner))
	if !strings.Contains(german.Body.String(), `<option value="title" selected>Zuletzt gespielt</option>`) {
		t.Fatalf("/?view=history&lang=de sort menu does not reuse the localized Recently played label")
	}
	movies := httptest.NewRecorder()
	fixture.mux.ServeHTTP(movies, snapshotRequest(t.Context(), http.MethodGet, "/?view=movies", owner))
	if !strings.Contains(movies.Body.String(), `<option value="title" selected>Title</option>`) || strings.Contains(movies.Body.String(), "Recently played</option>") {
		t.Fatalf("/?view=movies sort menu lost its Title label")
	}
}
