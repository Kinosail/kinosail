package metadata

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestChapterPresentationAndLookupBoundaries(t *testing.T) { //nolint:cyclop // One table exercises the chapter trust boundary without network side effects.
	t.Parallel()
	if got := (Chapter{Start: 62}).Timestamp(); got != "1:02" {
		t.Fatalf("short timestamp = %q", got)
	}
	if got := (Chapter{Start: 3662}).Timestamp(); got != "1:01:02" {
		t.Fatalf("long timestamp = %q", got)
	}
	if NewChapterProvider("") == nil {
		t.Fatal("default chapter provider was not created")
	}
	if NewChapterProvider("://bad") != nil {
		t.Fatal("invalid chapter provider was created")
	}
	local := []Chapter{{Title: "Named"}}
	for name, call := range map[string]func() []Chapter{
		"nil provider": func() []Chapter { return (*ChapterProvider)(nil).Chapters(t.Context(), library.Item{}, 1, local) },
		"non-finite duration": func() []Chapter {
			return NewChapterProvider("").Chapters(t.Context(), library.Item{}, math.NaN(), local)
		},
		"non-positive duration": func() []Chapter { return NewChapterProvider("").Chapters(t.Context(), library.Item{}, 0, local) },
		"named local chapters":  func() []Chapter { return NewChapterProvider("").Chapters(t.Context(), library.Item{}, 1, local) },
	} {
		t.Run(name, func(t *testing.T) {
			if got := call(); len(got) != 1 || got[0].Title != "Named" {
				t.Fatalf("local chapters = %#v", got)
			}
		})
	}
	if needsChapterDBLookup(local) || !needsChapterDBLookup(nil) || !needsChapterDBLookup([]Chapter{{Title: "Chapter 1"}}) {
		t.Fatal("chapter lookup policy mismatch")
	}
	for _, item := range []library.Item{
		{Episode: 1, ProviderIDs: map[string]string{"tvdb": "1"}},
		{Show: "Show", ProviderIDs: map[string]string{"tvdb": "1"}},
		{Show: "Show", Episode: 1, ProviderIDs: map[string]string{"tvdb": "bad"}},
		{Show: "Show", Episode: 1, ProviderIDs: map[string]string{"tvdb": "0"}},
	} {
		if _, ok := chaptersDBEpisodeID(item); ok {
			t.Fatalf("invalid chapter identity accepted: %#v", item)
		}
	}
}

func TestChapterProviderFetchFailuresAndEmptyResults(t *testing.T) { //nolint:cyclop // Remote failure classes must all fail closed.
	t.Parallel()
	clientError := &ChapterProvider{baseURL: "https://example.com", client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}}
	if chapters, found := clientError.fetch(t.Context(), "1", 60); found || chapters != nil {
		t.Fatalf("client failure = %#v, %v", chapters, found)
	}
	for name, response := range map[string]struct {
		status int
		body   string
	}{
		"not found":     {status: http.StatusNotFound},
		"bad status":    {status: http.StatusBadGateway},
		"invalid json":  {status: http.StatusOK, body: "{"},
		"too many sets": {status: http.StatusOK, body: fmt.Sprintf(`{"chapters":[%s]}`, strings.TrimSuffix(strings.Repeat(`{},`, maxChaptersDBSets+1), ","))},
		"no valid set":  {status: http.StatusOK, body: `{"chapters":[{"entries":[]}]}`},
	} {
		t.Run(name, func(t *testing.T) {
			provider := &ChapterProvider{baseURL: "https://example.com", client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return providerResponse(response.status, response.body), nil
			})}}
			chapters, found := provider.fetch(t.Context(), "1", 60)
			wantFound := name == "not found" || name == "no valid set"
			if found != wantFound || chapters != nil {
				t.Fatalf("fetch result = %#v, %v", chapters, found)
			}
		})
	}
	invalidRequest := &ChapterProvider{baseURL: "http://[::1", client: http.DefaultClient}
	if chapters, found := invalidRequest.fetch(t.Context(), "1", 60); found || chapters != nil {
		t.Fatalf("invalid request = %#v, %v", chapters, found)
	}
}

func TestChapterEntryAndTimeValidationEdges(t *testing.T) { //nolint:cyclop // Representative structural and grammar failures are individually asserted.
	t.Parallel()
	tooMany := make([]chaptersDBEntry, maxChaptersDBEntries+1)
	for name, entries := range map[string][]chaptersDBEntry{
		"empty":          nil,
		"too many":       tooMany,
		"non-increasing": {{Time: "00:00:01", Name: "One"}, {Time: "00:00:01", Name: "Two"}},
		"past duration":  {{Time: "00:01:00", Name: "Late"}},
		"blank title":    {{Time: "00:00:00", Name: " "}},
		"long title":     {{Time: "00:00:00", Name: strings.Repeat("x", maxChapterTitleBytes+1)}},
		"control title":  {{Time: "00:00:00", Name: "bad\nname"}},
	} {
		t.Run(name, func(t *testing.T) {
			if chapters, ok := chaptersFromDBEntries(entries, 60); ok || chapters != nil {
				t.Fatalf("invalid entries accepted: %#v", chapters)
			}
		})
	}
	for _, duration := range []float64{0, math.NaN()} {
		if _, ok := chaptersFromDBEntries([]chaptersDBEntry{{Time: "00:00:00", Name: "Intro"}}, duration); ok {
			t.Fatalf("invalid duration %v accepted", duration)
		}
	}
	for _, value := range []string{"00:00", "x:00:00", "00:xx:00", "00:00:xx", "00:60:00", "00:00:60", "00:00:00.", "00:00:00.1234", "00:00:00.x"} {
		if _, ok := parseChapterDBTime(value); ok {
			t.Fatalf("invalid chapter time %q accepted", value)
		}
	}
}

func TestChapterLookupCoalescesWaitersAndHonorsCancellation(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	provider := &ChapterProvider{
		baseURL: "https://example.com",
		client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			once.Do(func() { close(started) })
			select {
			case <-release:
				return providerResponse(http.StatusOK, `{"chapters":[{"entries":[{"time":"00:00:00","name":"Intro"}]}]}`), nil
			case <-request.Context().Done():
				return nil, context.Cause(request.Context())
			}
		})},
		cached: make(map[string]chapterLookup), calls: make(map[string]*chapterLookupCall),
	}
	first := make(chan []Chapter, 1)
	go func() {
		chapters, _ := provider.lookup(t.Context(), "1", 60)
		first <- chapters
	}()
	<-started
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if chapters, found := provider.lookup(canceled, "1", 60); found || chapters != nil {
		t.Fatalf("canceled waiter = %#v, %v", chapters, found)
	}
	waiter := make(chan []Chapter, 1)
	go func() {
		chapters, _ := provider.lookup(t.Context(), "1", 60)
		waiter <- chapters
	}()
	close(release)
	for name, result := range map[string][]Chapter{"leader": <-first, "waiter": <-waiter} {
		if len(result) != 1 || result[0].Title != "Intro" {
			t.Fatalf("%s result = %#v", name, result)
		}
	}
}

func TestChapterLookupReturnsCompletedInFlightResult(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	close(done)
	provider := &ChapterProvider{
		cached: make(map[string]chapterLookup),
		calls: map[string]*chapterLookupCall{
			"1": {done: done, chapters: []Chapter{{Title: "Intro"}}, found: true},
		},
	}
	chapters, found := provider.lookup(t.Context(), "1", 60)
	if !found || len(chapters) != 1 || chapters[0].Title != "Intro" {
		t.Fatalf("completed lookup = %#v, %v", chapters, found)
	}
}
