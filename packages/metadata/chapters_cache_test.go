package metadata_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestChaptersDBRemembersProviderFailuresBriefly(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(writer, "unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(provider.Close)
	client := metadata.NewChapterProvider(provider.URL + "/api/v1")
	item := library.Item{Show: "Example", Episode: 1, ProviderIDs: map[string]string{"tvdb": "123"}}
	for range 3 {
		if chapters := client.Chapters(t.Context(), item, 120, nil); len(chapters) != 0 {
			t.Fatalf("chapters = %#v", chapters)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("ChaptersDB calls after failures = %d, want 1", calls.Load())
	}
}

func TestChaptersDBCachesResultsPerDuration(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = writer.Write([]byte(`{"chapters":[{"entries":[{"time":"00:00:00.000","name":"Cold open"},{"time":"00:01:30.500","name":"The case"}]}]}`))
	}))
	t.Cleanup(provider.Close)
	client := metadata.NewChapterProvider(provider.URL + "/api/v1")
	item := library.Item{Show: "Example", Episode: 1, ProviderIDs: map[string]string{"tvdb": "123"}}
	if chapters := client.Chapters(t.Context(), item, 60, nil); len(chapters) != 0 {
		t.Fatalf("short-media chapters = %#v", chapters)
	}
	if chapters := client.Chapters(t.Context(), item, 120, nil); len(chapters) != 2 {
		t.Fatalf("full-length chapters = %#v", chapters)
	}
	if chapters := client.Chapters(t.Context(), item, 120, nil); len(chapters) != 2 || calls.Load() != 2 {
		t.Fatalf("cached chapters = %#v after %d calls", chapters, calls.Load())
	}
}
