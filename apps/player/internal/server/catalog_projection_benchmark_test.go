package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkNativeCatalogProfileProjection(b *testing.B) { //nolint:gocognit // Keep the public view, profile, and cardinality matrix together.
	for _, count := range []int{10_000, 100_000} {
		for _, owner := range []bool{true, false} {
			b.Run(fmt.Sprintf("%d/owner=%t", count, owner), func(b *testing.B) {
				items, progress, lists, profile := projectionBenchmarkFixture(count, owner)
				index := memoryLibraryIndex(items, true)
				handler := projectionBenchmarkHandler(index, progress, lists)
				seed := profile
				seed.Owner = true
				response := httptest.NewRecorder()
				handler(response, projectionBenchmarkRequest(b, count, "all", seed))
				projectionBenchmarkResponse(b, response, count, count, true)
				for _, view := range []string{"all", "movies", "list", "unwatched", "history", "exact"} {
					b.Run(view, func(b *testing.B) {
						request := projectionBenchmarkRequest(b, count, view, profile)
						b.ReportAllocs()
						var response *httptest.ResponseRecorder
						for b.Loop() {
							response = httptest.NewRecorder()
							handler(response, request)
							if response.Code != http.StatusOK {
								b.Fatal(response.Code)
							}
						}
						projectionBenchmarkResponse(b, response, projectionBenchmarkTotal(count, owner, view), count, owner)
						b.ReportMetric(float64(response.Body.Len()), "response-bytes")
					})
				}
			})
		}
	}
}

func BenchmarkNativeCatalogProfileCold(b *testing.B) {
	const count = 100_000
	for _, owner := range []bool{true, false} {
		for _, view := range []string{"movies", "list", "exact"} {
			b.Run(fmt.Sprintf("owner=%t/%s", owner, view), func(b *testing.B) {
				items, progress, lists, profile := projectionBenchmarkFixture(count, owner)
				request := projectionBenchmarkRequest(b, count, view, profile)
				b.ReportAllocs()
				var response *httptest.ResponseRecorder
				for b.Loop() {
					b.StopTimer()
					index := memoryLibraryIndex(items, true)
					handler := projectionBenchmarkHandler(index, progress, lists)
					response = httptest.NewRecorder()
					b.StartTimer()
					handler(response, request)
					if response.Code != http.StatusOK {
						b.Fatal(response.Code)
					}
				}
				projectionBenchmarkResponse(b, response, projectionBenchmarkTotal(count, owner, view), count, owner)
				b.ReportMetric(float64(response.Body.Len()), "response-bytes")
			})
		}
	}
}

func projectionBenchmarkFixture(count int, owner bool) ([]library.Item, *progressStore, *listStore, viewerProfile) {
	profile := viewerProfile{ID: strings.Repeat("a", 26), Owner: owner, Rating: "all", Libraries: []string{"library-b"}}
	items := make([]library.Item, count)
	values := make(map[string]playbackState, count/5)
	lists := newListStore("")
	for position := range items {
		id := fmt.Sprintf("%016x", position)
		name := "library-a"
		if position%2 == 1 {
			name = "library-b"
		}
		items[position] = library.Item{ID: id, Library: name, Kind: "video", Title: fmt.Sprintf("Film %06d", position), Year: "2026"}
		if position%10 < 2 {
			key := profile.ID + ":" + id
			values[key] = playbackState{Seconds: 30, Watched: position%20 < 2, Updated: time.Unix(1000+int64(position), 0)}
			lists.values[key] = true
			if owner && position%40 < 2 {
				values[id] = values[key]
				delete(values, key)
			}
		}
		if !owner && position%40 == 3 {
			values[id] = playbackState{Watched: true, Updated: time.Unix(1, 0)}
		}
	}
	progress := newProgressStore("")
	progress.Replace(values)
	return items, progress, lists, profile
}

func projectionBenchmarkHandler(index *libraryIndex, progress *progressStore, lists *listStore) http.HandlerFunc {
	return catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
		return browseLibrary(request, index, progress, lists)
	}, progress.ClientItem)
}

func projectionBenchmarkRequest(b *testing.B, count int, view string, profile viewerProfile) *http.Request {
	target := "/api/v1/library?view=" + view
	if view == "exact" {
		target = "/api/v1/library?view=movies&q=Film+" + fmt.Sprintf("%06d", count-1)
	}
	request := httptest.NewRequestWithContext(b.Context(), "GET", target, nil)
	return request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, profile))
}

func projectionBenchmarkTotal(count int, owner bool, view string) int {
	visible := count
	if !owner {
		visible /= 2
	}
	switch view {
	case "list", "history":
		return visible / 5
	case "unwatched":
		return visible * 9 / 10
	case "exact":
		return 1
	default:
		return visible
	}
}

func projectionBenchmarkResponse(b *testing.B, response *httptest.ResponseRecorder, total, count int, owner bool) { //nolint:cyclop // One response contract checks counts, IDs, visibility, and exact-search membership.
	b.Helper()
	var page struct {
		Total int
		Items []struct{ ID string }
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK || page.Total != total || len(page.Items) != min(total, 100) {
		b.Fatalf("response status=%d total=%d items=%d want=%d err=%v", response.Code, page.Total, len(page.Items), total, err)
	}
	for _, item := range page.Items {
		position, err := strconv.ParseUint(item.ID, 16, 64)
		if err != nil || len(item.ID) != 16 || position >= uint64(count) || !owner && position%2 != 1 || total == 1 && position != uint64(count-1) { //nolint:gosec // count is a positive 10,000 or 100,000 fixture constant.
			b.Fatalf("unexpected or denied item %q: %v", item.ID, err)
		}
	}
}
