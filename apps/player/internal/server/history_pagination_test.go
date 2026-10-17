package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type historyPageBody struct {
	Items  []struct{ ID string } `json:"items"`
	Total  *int                  `json:"total"`
	Offset *int                  `json:"offset"`
	Limit  *int                  `json:"limit"`
	Error  string                `json:"error"`
}

func historyPage(t *testing.T, handler http.Handler, target string, profile viewerProfile) (int, historyPageBody, string) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, snapshotRequest(t.Context(), http.MethodGet, target, profile))
	var body historyPageBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s body = %q: %v", target, response.Body.String(), err)
	}
	return response.Code, body, response.Body.String()
}

func historyIDs(body historyPageBody) []string {
	ids := make([]string, 0, len(body.Items))
	for _, item := range body.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestPlaybackHistoryAPIPagesMostRecentFirstWithinViewerAccess(t *testing.T) {
	items := snapshotFixtureItems(12)
	fixture := newSnapshotFixture(t, items)
	owner := viewerProfile{ID: "owner", Owner: true}
	family := viewerProfile{ID: "family", Rating: "family", Libraries: []string{"all"}}
	watched := []int{7, 2, 9, 0, 4, 3, 6}
	for _, position := range watched {
		for _, profile := range []viewerProfile{owner, family} {
			if err := fixture.progress.Set(snapshotRequest(t.Context(), http.MethodPut, "/", profile), items[position].ID, 30, nil); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	recent := make([]string, 0, len(watched))
	for _, position := range slices.Backward(watched) {
		recent = append(recent, items[position].ID)
	}

	status, full, raw := historyPage(t, fixture.mux, "/api/v1/history", owner)
	if status != http.StatusOK || !slices.Equal(historyIDs(full), recent) || full.Total != nil || full.Offset != nil || full.Limit != nil {
		t.Fatalf("unpaged history = %d %q, want only items %v", status, raw, recent)
	}
	for _, test := range []struct {
		target        string
		want          []string
		offset, limit int
	}{
		{"/api/v1/history?limit=3", recent[:3], 0, 3},
		{"/api/v1/history?offset=3&limit=3", recent[3:6], 3, 3},
		{"/api/v1/history?offset=6&limit=3", recent[6:], 6, 3},
		{"/api/v1/history?offset=2", recent[2:], 2, 100},
		{"/api/v1/history?offset=7&limit=200", nil, 7, 200},
		{"/api/v1/history?offset=1000000&limit=1", nil, 1000000, 1},
	} {
		status, page, raw := historyPage(t, fixture.mux, test.target, owner)
		if status != http.StatusOK || !slices.Equal(historyIDs(page), test.want) || page.Total == nil || *page.Total != len(recent) || *page.Offset != test.offset || *page.Limit != test.limit {
			t.Fatalf("%s = %d %q, want %v of %d", test.target, status, raw, test.want, len(recent))
		}
	}

	familyRecent := make([]string, 0, len(recent))
	for _, id := range recent {
		position := slices.IndexFunc(items, func(item library.Item) bool { return item.ID == id })
		if canView(family, items[position]) {
			familyRecent = append(familyRecent, id)
		}
	}
	if len(familyRecent) < 3 || len(familyRecent) == len(recent) {
		t.Fatalf("family fixture history = %v, want a strict subset of %v", familyRecent, recent)
	}
	status, restricted, raw := historyPage(t, fixture.mux, "/api/v1/history?limit=2", family)
	if status != http.StatusOK || !slices.Equal(historyIDs(restricted), familyRecent[:2]) || restricted.Total == nil || *restricted.Total != len(familyRecent) {
		t.Fatalf("restricted history page = %d %q, want %v of %d", status, raw, familyRecent[:2], len(familyRecent))
	}

	for _, target := range []string{
		"/api/v1/history?limit=0", "/api/v1/history?limit=201", "/api/v1/history?limit=-1", "/api/v1/history?limit=ten",
		"/api/v1/history?offset=-1", "/api/v1/history?offset=1000001", "/api/v1/history?offset=1&offset=2",
		"/api/v1/history?limit=1&limit=2", "/api/v1/history?limit=00000000001", "/api/v1/history?offset=" + strings.Repeat("9", 64),
	} {
		status, rejected, raw := historyPage(t, fixture.mux, target, owner)
		if status != http.StatusBadRequest || rejected.Error == "" || len(rejected.Items) != 0 {
			t.Fatalf("%s = %d %q, want 400 error", target, status, raw)
		}
	}
	status, after, raw := historyPage(t, fixture.mux, "/api/v1/history", owner)
	if status != http.StatusOK || !slices.Equal(historyIDs(after), recent) {
		t.Fatalf("history after rejected pages = %d %q, want %v", status, raw, recent)
	}
}
