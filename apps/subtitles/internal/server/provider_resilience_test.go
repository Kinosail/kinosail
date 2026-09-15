package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleSearchesStartTogether(t *testing.T) {
	var arrived atomic.Int32
	all := make(chan struct{})
	fixture := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if arrived.Add(1) == 3 {
			close(all)
		}
		select {
		case <-all:
		case <-request.Context().Done():
			return
		}
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer fixture.Close()
	provider := newSubtitleProvider(SubtitleConfig{
		URL: fixture.URL + "/subdl", APIKey: "fixture",
		OpenSubtitles: OpenSubtitlesConfig{URL: fixture.URL + "/open", APIKey: "fixture", Username: "fixture", Password: "fixture"},
		SubSource:     SubSourceConfig{URL: fixture.URL + "/source", APIKey: "fixture", PersonalUse: true},
	}, t.TempDir(), "", nil, nil, "")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	candidates, configured, failed := provider.subtitleCandidates(ctx, library.Item{ID: "film", Kind: "video", Title: "Film", Path: "/missing/film.mkv"}, "en")
	if ctx.Err() != nil || len(candidates) != 0 || configured != 3 || failed != 3 || arrived.Load() != 3 {
		t.Fatalf("parallel search = %d candidates, %d configured, %d failed, %d arrived, %v", len(candidates), configured, failed, arrived.Load(), ctx.Err())
	}
}

func TestOpenSubtitlesLoginDoesNotHoldSessionMutexAcrossNetwork(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	fixture := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-request.Context().Done():
			return
		}
		_, _ = writer.Write([]byte(`{"status":200,"token":"fixture-token"}`))
	}))
	defer fixture.Close()
	provider := newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: fixture.URL, APIKey: "fixture", Username: "fixture", Password: "fixture"})
	done := make(chan error, 1)
	go func() { _, _, err := provider.session(t.Context()); done <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("login did not start")
	}
	cached := make(chan struct{})
	go func() { provider.cachedSession(); close(cached) }()
	select {
	case <-cached:
	case <-time.After(time.Second):
		once.Do(func() { close(release) })
		t.Fatal("cached session blocked behind login network")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := provider.session(ctx); err == nil {
		t.Fatal("canceled waiter did not stop")
	}
	once.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestTemporaryLoginFailureUsesProviderRetryInsteadOfOneHour(t *testing.T) {
	var calls atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte(`{"status":200,"token":"fixture-token"}`))
	}))
	defer fixture.Close()
	provider := newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: fixture.URL, APIKey: "fixture", Username: "fixture", Password: "fixture"})
	now := time.Now()
	provider.health.now = func() time.Time { return now }
	if _, _, err := provider.session(t.Context()); err == nil {
		t.Fatal("temporary failure accepted")
	}
	now = now.Add(2 * time.Minute)
	token, _, err := provider.session(t.Context())
	if err != nil || token != "fixture-token" || calls.Load() != 2 {
		t.Fatalf("retry = %q, %v, %d", token, err, calls.Load())
	}
}
