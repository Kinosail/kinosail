package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

// No all-title request precedes the native entry routes in this workload.
func BenchmarkNativeMixedCatalog(b *testing.B) { //nolint:gocognit,cyclop // Keep route, profile seed, cardinality, and cold/warm controls in one fixture.
	for _, count := range []int{10_000, 100_000} {
		for _, owner := range []bool{true, false} {
			for _, view := range []string{"movies", "shows", "music"} {
				for _, cold := range []bool{false, true} {
					seeds := []bool{owner}
					if !owner && !cold {
						seeds = append(seeds, true)
					}
					for _, seedOwner := range seeds {
						b.Run(fmt.Sprintf("%d/owner=%t/%s/cold=%t/seed-owner=%t", count, owner, view, cold, seedOwner), func(b *testing.B) {
							items, progress, lists, profile := projectionBenchmarkFixture(count, owner)
							mixedBenchmarkItems(items)
							request := projectionBenchmarkRequest(b, count, view, profile)
							handler := projectionBenchmarkHandler(memoryLibraryIndex(items, true), progress, lists)
							if seedOwner != owner {
								seedProfile := profile
								seedProfile.Owner = seedOwner
								response := httptest.NewRecorder()
								handler(response, projectionBenchmarkRequest(b, count, view, seedProfile))
								mixedBenchmarkResponse(b, response, count, seedOwner, view)
							}
							seed := httptest.NewRecorder()
							handler(seed, request)
							mixedBenchmarkResponse(b, seed, count, owner, view)
							var response *httptest.ResponseRecorder
							b.ReportAllocs()
							for b.Loop() {
								if cold {
									b.StopTimer()
									index := memoryLibraryIndex(items, true)
									handler = projectionBenchmarkHandler(index, progress, lists)
									response = httptest.NewRecorder()
									b.StartTimer()
								} else {
									response = httptest.NewRecorder()
								}
								handler(response, request)
								if response.Code != http.StatusOK {
									b.Fatal(response.Code)
								}
							}
							mixedBenchmarkResponse(b, response, count, owner, view)
							if response.Body.String() != seed.Body.String() {
								b.Fatal("entry response changed across repeated reads")
							}
							b.ReportMetric(float64(response.Body.Len()), "response-bytes")
							b.Logf("response-sha256=%x", sha256.Sum256(response.Body.Bytes()))
						})
					}
				}
			}
		}
	}
}

func mixedBenchmarkItems(items []library.Item) {
	for position := range items {
		item := &items[position]
		item.Title = fmt.Sprintf("%s %06d", []string{"Ängel", "Alpha", "Åland", "Élan"}[position%4], len(items)-position)
		item.Library = "library-a"
		if position/4%2 == 1 {
			item.Library = "library-b"
		}
		switch position % 4 {
		case 2:
			item.Show = fmt.Sprintf("show-%06d", position/16)
			item.ShowTitle = fmt.Sprintf("Series %06d", len(items)/16-position/16)
			item.Season, item.Episode = 1, position/4%4+1
		case 3:
			item.Kind = "audio"
		}
	}
}

func mixedBenchmarkResponse(b *testing.B, response *httptest.ResponseRecorder, count int, owner bool, view string) { //nolint:gocognit,cyclop // One public response contract checks counts, permitted IDs, and intrinsic kinds.
	b.Helper()
	total := count / 2
	if view == "music" {
		total = count / 4
	}
	if !owner {
		total /= 2
	}
	if view == "shows" {
		total = count / 16
	}
	var page struct {
		Total int
		Items []struct{ ID string }
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK || page.Total != total || len(page.Items) != min(100, total) {
		b.Fatalf("entry status=%d total=%d count=%d want=%d err=%v", response.Code, page.Total, len(page.Items), total, err)
	}
	for _, item := range page.Items {
		position, err := strconv.ParseUint(item.ID, 16, 64)
		if err != nil || position >= uint64(count) || !owner && position/4%2 != 1 { //nolint:gosec // count is a positive 10,000 or 100,000 fixture constant.
			b.Fatalf("unexpected or denied entry %q: %v", item.ID, err)
		}
		switch view {
		case "movies":
			if position%4 > 1 {
				b.Fatalf("non-movie entry %q", item.ID)
			}
		case "music":
			if position%4 != 3 {
				b.Fatalf("non-music entry %q", item.ID)
			}
		case "shows":
			if position%4 != 2 {
				b.Fatalf("non-show entry %q", item.ID)
			}
		}
	}
}
