package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func pagedSubtitleFixture(t *testing.T, count int) *subtitleManager {
	t.Helper()
	manager, _ := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
	manager.probe = nil
	items := make([]library.Item, 0, count)
	for i := 0; i < count; i++ {
		item := library.Item{ID: fmt.Sprintf("%016x", i+1), Kind: "video", Title: fmt.Sprintf("Film %05d", i), Path: fmt.Sprintf("/missing/Film-%05d.mkv", i), Added: time.Unix(int64(i+1), 0)}
		if i%2 == 0 {
			item.Subtitles = []string{fmt.Sprintf("/missing/Film-%05d.en.srt", i)}
		}
		items = append(items, item)
	}
	manager.index = memoryLibraryIndex(items, true)
	return manager
}

func readSubtitlePage(t *testing.T, manager *subtitleManager, query string) subtitleDashboardData {
	t.Helper()
	recorder := httptest.NewRecorder()
	manager.statusAPI(recorder, ownerRequest("/api/v1/subtitle-library"+query))
	var data subtitleDashboardData
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &data) != nil {
		t.Fatalf("page = %d %s", recorder.Code, recorder.Body.String())
	}
	return data
}

func TestSubtitleLibraryBoundsLargeResponsesAndKeepsGlobalCounts(t *testing.T) {
	t.Parallel()
	manager := pagedSubtitleFixture(t, 4000)
	page := readSubtitlePage(t, manager, "?view=library")
	if len(page.Items) != 40 || page.Total != 4000 || page.Ready != 2000 || page.Wanted != 2000 || page.Pages != 100 || page.Matched != 4000 || page.Start != 1 || page.End != 40 {
		t.Fatalf("incorrect first page: %#v", page)
	}
	recorder := httptest.NewRecorder()
	manager.dashboard(recorder, ownerRequest("/?view=library"))
	if strings.Count(recorder.Body.String(), `class="subtitle-file"`) != 40 || recorder.Body.Len() > 160<<10 {
		t.Fatalf("unbounded HTML: %d bytes", recorder.Body.Len())
	}
	overview := readSubtitlePage(t, manager, "")
	if len(overview.Items) != 6 || overview.Matched != 2000 || overview.NextURL != "" {
		t.Fatalf("overview is not a bounded preview: %#v", overview)
	}
}

func TestSubtitleLibraryPagesHaveStableOrderAndClampAfterRemoval(t *testing.T) {
	t.Parallel()
	manager := pagedSubtitleFixture(t, 95)
	seen := make(map[string]bool)
	for number := 1; number <= 3; number++ {
		page := readSubtitlePage(t, manager, fmt.Sprintf("?view=library&page=%d", number))
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate across pages: %s", item.ID)
			}
			seen[item.ID] = true
		}
	}
	if len(seen) != 95 {
		t.Fatalf("only %d files were reachable", len(seen))
	}
	last := readSubtitlePage(t, manager, "?view=library&page=1000000")
	if last.Page != 3 || len(last.Items) != 15 || last.Start != 81 || last.End != 95 || last.NextURL != "" || last.PreviousURL == "" {
		t.Fatalf("invalid final page: %#v", last)
	}
	manager.index = memoryLibraryIndex(nil, true)
	empty := readSubtitlePage(t, manager, "?view=library&page=3")
	if empty.Page != 1 || empty.Pages != 1 || empty.Start != 0 || empty.End != 0 || len(empty.Items) != 0 {
		t.Fatalf("empty page = %#v", empty)
	}
}

func TestSubtitleLibraryFiltersBeforePaginationAndPreservesNavigation(t *testing.T) {
	t.Parallel()
	manager := pagedSubtitleFixture(t, 100)
	page := readSubtitlePage(t, manager, "?view=library&status=wanted&kind=movie&sort=modified&q=Film&page=1")
	if page.Matched != 50 || len(page.Items) != 40 || page.Items[0].ID != "0000000000000064" {
		t.Fatalf("filtered page = %#v", page)
	}
	next, err := url.Parse(page.NextURL)
	if err != nil || next.Query().Get("status") != "wanted" || next.Query().Get("kind") != "movie" || next.Query().Get("sort") != "modified" || next.Query().Get("q") != "Film" || next.Query().Get("page") != "2" {
		t.Fatalf("filters lost: %q", page.NextURL)
	}
	for _, row := range page.Items {
		if row.Ready || row.MediaKind != "movie" {
			t.Fatalf("wrong filtered file: %#v", row)
		}
	}
	wanted := readSubtitlePage(t, manager, "?view=wanted")
	if wanted.Matched != 50 || len(wanted.Items) != 40 {
		t.Fatal("wanted view did not paginate missing files")
	}
}

func TestSubtitleLibraryUsesShowIdentityAndDeterministicTies(t *testing.T) {
	t.Parallel()
	manager := pagedSubtitleFixture(t, 0)
	items := []library.Item{
		{ID: "0000000000000003", Kind: "video", Title: "Episode", Show: "Folder", ShowTitle: "Severance", Season: 2, Episode: 1, Path: "/missing/three.mkv"},
		{ID: "0000000000000002", Kind: "video", Title: "Episode", Show: "Folder", ShowTitle: "Severance", Season: 1, Episode: 1, Path: "/missing/two.mkv"},
		{ID: "0000000000000001", Kind: "video", Title: "Episode", Show: "Folder", ShowTitle: "Severance", Season: 1, Episode: 1, Path: "/missing/one.mkv"},
	}
	manager.index = memoryLibraryIndex(items, true)
	page := readSubtitlePage(t, manager, "?view=library&kind=episode&q=severance")
	if page.Matched != 3 || page.Items[0].ID != items[2].ID || page.Items[2].ID != items[0].ID || page.Items[0].Title != "Severance" || page.Items[0].Context != "S01E01 · Episode" {
		t.Fatalf("show identity/order = %#v", page.Items)
	}
}

func TestSubtitleLibraryCountsRespectViewerVisibility(t *testing.T) {
	t.Parallel()
	manager := pagedSubtitleFixture(t, 0)
	manager.index = memoryLibraryIndex([]library.Item{
		{ID: "0000000000000001", Kind: "video", Title: "Public", Path: "/one", Library: "family"},
		{ID: "0000000000000002", Kind: "video", Title: "Private", Path: "/two", Library: "private"},
	}, true)
	request := ownerRequest("/api/v1/subtitle-library?view=library")
	request = request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, viewerProfile{ID: "viewer", Rating: "all", Libraries: []string{"family"}}))
	response := httptest.NewRecorder()
	manager.statusAPI(response, request)
	var page subtitleDashboardData
	if json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Total != 1 || page.Matched != 1 || len(page.Items) != 1 || strings.Contains(response.Body.String(), "Private") {
		t.Fatalf("visibility leaked: %s", response.Body.String())
	}
}

func TestSubtitleLibraryRejectsMalformedFiltersBeforeAnyWork(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"page=0", "page=-1", "page=+1", "page=01", "page=1.0", "page=", "page=1000001", "page=999999999999999999999", "page=1&page=2",
		"kind=series", "kind=", "kind=movie&kind=episode", "sort=random", "sort=", "status=unknown", "status=", "status=ready&status=wanted",
		"view=wanted&status=ready", "view=summary&status=checking", "view=summary&page=2", "q=%FF", "q=%", "q=a;b", "q=" + strings.Repeat("x", 129), "q=" + strings.Repeat("x", 1025), "unexpected=1",
	}
	for _, query := range invalid {
		t.Run(query[:min(len(query), 40)], func(t *testing.T) {
			manager := &subtitleManager{} // Validation must precede all settings, index, provider, and file access.
			for _, handler := range []http.HandlerFunc{manager.dashboard, manager.statusAPI} {
				request := ownerRequest("/")
				request.URL.RawQuery = query
				recorder := httptest.NewRecorder()
				handler(recorder, request)
				if recorder.Code != http.StatusBadRequest || recorder.Flushed {
					t.Fatalf("invalid query accepted: %q, status %d", query, recorder.Code)
				}
			}
		})
	}
}

func TestSubtitleLibraryOrdersLongSeasonsNumerically(t *testing.T) {
	t.Parallel()
	manager := pagedSubtitleFixture(t, 0)
	manager.index = memoryLibraryIndex([]library.Item{
		{ID: "0000000000000001", Kind: "video", ShowTitle: "Show", Season: 1, Episode: 100},
		{ID: "0000000000000002", Kind: "video", ShowTitle: "Show", Season: 1, Episode: 11},
	}, true)
	page := readSubtitlePage(t, manager, "?view=library")
	if len(page.Items) != 2 || page.Items[0].ID != "0000000000000002" {
		t.Fatalf("episode order = %#v", page.Items)
	}
}
