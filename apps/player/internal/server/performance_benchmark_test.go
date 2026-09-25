package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func ownerRequest(target string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), "GET", target, nil)
	return request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, viewerProfile{ID: "owner", Owner: true}))
}

func TestLargeLibraryWebResponseIsBounded(t *testing.T) {
	items := benchmarkLibraryItems()
	index := memoryLibraryIndex(items, true)
	response := httptest.NewRecorder()
	showHome(index, newProgressStore(""), newListStore(""), newSettingsStore("", "", "", nil), nil, false)(response, ownerRequest("/?view=movies"))
	if cards := strings.Count(response.Body.String(), `/item/`); response.Code != 200 || cards != catalog.DefaultPageSize || response.Body.Len() > 27_000 || !strings.Contains(response.Body.String(), `offset=100`) {
		t.Fatalf("large Library page = status %d, cards %d, bytes %d", response.Code, cards, response.Body.Len())
	}
}

func TestLargeLetterBucketRemainsBoundedAndPageable(t *testing.T) {
	items := benchmarkLibraryItems()
	index := memoryLibraryIndex(items, true)
	handler := showHome(index, newProgressStore(""), newListStore(""), newSettingsStore("", "", "", nil), nil, false)
	response := httptest.NewRecorder()
	handler(response, ownerRequest("/?view=movies&letter=M"))
	if cards := strings.Count(response.Body.String(), `/item/`); response.Code != http.StatusOK || cards != catalog.DefaultPageSize || !strings.Contains(response.Body.String(), `letter=M&amp;limit=100&amp;offset=100`) {
		t.Fatalf("large letter page = status %d, cards %d, bytes %d", response.Code, cards, response.Body.Len())
	}
	next := httptest.NewRecorder()
	handler(next, ownerRequest("/?view=movies&letter=M&limit=100&offset=100"))
	if cards := strings.Count(next.Body.String(), `/item/`); next.Code != http.StatusOK || cards != catalog.DefaultPageSize {
		t.Fatalf("next letter page = status %d, cards %d", next.Code, cards)
	}
}

func BenchmarkLargeLibraryBrowse(b *testing.B) {
	items := benchmarkLibraryItems()
	index := memoryLibraryIndex(items, true)
	handler := showHome(index, newProgressStore(""), newListStore(""), newSettingsStore("", "", "", nil), nil, false)
	request := ownerRequest("/?view=movies")
	b.ReportAllocs()
	responseBytes := 0
	for b.Loop() {
		response := httptest.NewRecorder()
		handler(response, request)
		responseBytes = response.Body.Len()
	}
	b.ReportMetric(float64(responseBytes), "response-bytes")
}

func BenchmarkLargeLibraryKnownTitleNavigation(b *testing.B) {
	items := benchmarkLibraryItems()
	index := memoryLibraryIndex(items, true)
	handler := showHome(index, newProgressStore(""), newListStore(""), newSettingsStore("", "", "", nil), nil, false)
	for name, target := range map[string]string{"search": "/?view=movies&q=Movie+9999", "letter": "/?view=movies&letter=M"} {
		b.Run(name, func(b *testing.B) {
			request := ownerRequest(target)
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				handler(response, request)
				if response.Code != http.StatusOK {
					b.Fatal(response.Code)
				}
			}
		})
	}
}

func benchmarkLibraryItems() []library.Item {
	items := make([]library.Item, 10_000)
	for index := range items {
		id := strconv.Itoa(index)
		items[index] = library.Item{ID: id, Kind: "video", Title: "Movie " + id, Year: "2026"}
	}
	return items
}

func BenchmarkSaveProgressJSON10K(b *testing.B) {
	values := make(map[string]playbackState, 10_000)
	for index := range 10_000 {
		values[strconv.Itoa(index)] = playbackState{Seconds: float64(index)}
	}
	store := newProgressStore(b.TempDir())
	store.Replace(values)
	b.ReportAllocs()
	for b.Loop() {
		if _, _, _, err := store.Update("0", func(state playbackState) (playbackState, bool, error) {
			state.Seconds++
			return state, true, nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSaveProgress10K(b *testing.B) {
	values := make(map[string]playbackState, 10_000)
	for index := range 10_000 {
		values[strconv.Itoa(index)] = playbackState{Seconds: float64(index)}
	}
	directory := b.TempDir()
	stateDB, err := database.Open(directory, true)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = stateDB.Close() })
	store := newProgressStore(directory, stateDB)
	store.Replace(values)
	b.ReportAllocs()
	for b.Loop() {
		if _, _, _, err := store.Update("0", func(state playbackState) (playbackState, bool, error) {
			state.Seconds++
			return state, true, nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}
