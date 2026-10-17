package mediashares

import (
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type heldResponse struct {
	header  http.Header
	status  int
	started chan struct{}
	resume  chan struct{}
	done    chan struct{}
	once    sync.Once
}

func (response *heldResponse) Header() http.Header { return response.header }

func (response *heldResponse) WriteHeader(status int) { response.status = status }

func (response *heldResponse) Write(data []byte) (int, error) {
	response.once.Do(func() { close(response.started) })
	<-response.resume
	return len(data), nil
}

func (response *heldResponse) finish() {
	close(response.resume)
	<-response.done
}

func holdStream(t *testing.T, store *Store, token string) *heldResponse {
	t.Helper()
	response := &heldResponse{header: make(http.Header), started: make(chan struct{}), resume: make(chan struct{}), done: make(chan struct{})}
	request := rangedMediaRequest(t, token, "bytes=0-")
	go func() {
		defer close(response.done)
		store.Serve(response, request)
	}()
	select {
	case <-response.started:
	case <-response.done:
		t.Fatalf("held stream finished early with status %d", response.status)
	case <-time.After(5 * time.Second):
		t.Fatal("held stream did not start")
	}
	if response.status != http.StatusPartialContent {
		t.Fatalf("held stream status = %d", response.status)
	}
	return response
}

func rangedMediaRequest(t *testing.T, token, byteRange string) *http.Request {
	t.Helper()
	request := capabilityRequest(t, token)
	request.URL.Path = "/share/media/item"
	request.Header.Set("Range", byteRange)
	request.SetPathValue("id", "item")
	return request
}

func serveRange(t *testing.T, store *Store, token, byteRange string) int {
	t.Helper()
	response := httptest.NewRecorder()
	store.Serve(response, rangedMediaRequest(t, token, byteRange))
	return response.Code
}

func deviceLimitStore(t *testing.T, clock *time.Time) (*Store, *memoryState) {
	t.Helper()
	media := filepath.Join(t.TempDir(), "movie.mp4")
	if err := os.WriteFile(media, []byte("movie bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := new(memoryState)
	return New(t.TempDir(), Dependencies{
		Load: state.load, Persist: state.persist,
		Find:     func(id string) (library.Item, bool) { return library.Item{ID: id, Path: media}, id == "item" },
		Snapshot: func() ([]library.Item, error) { return []library.Item{{ID: "item", Path: media}}, nil },
		Now:      func() time.Time { return *clock }, Random: rand.Reader,
	}), state
}

func TestOneDeviceStreamsOverlappingRangesWithinItsDeviceLimit(t *testing.T) {
	clock := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	store, _ := deviceLimitStore(t, &clock)
	_, claim, err := store.Create([]string{"item"}, 24*time.Hour, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	device, _, err := store.Claim(claim, "Safari")
	if err != nil {
		t.Fatal(err)
	}
	held := make([]*heldResponse, 0, maxStreamsPerDevice)
	for range maxStreamsPerDevice {
		held = append(held, holdStream(t, store, device))
	}
	if status := serveRange(t, store, device, "bytes=2-"); status != http.StatusNotFound {
		t.Fatalf("request beyond the per-device stream bound = %d", status)
	}
	held[0].finish()
	if status := serveRange(t, store, device, "bytes=4-"); status != http.StatusPartialContent {
		t.Fatalf("overlapping range after one stream closed = %d", status)
	}
	for _, response := range held[1:] {
		response.finish()
	}
}

func TestSecondDeviceIsRefusedWithoutSideEffectsWhileFirstDeviceStreams(t *testing.T) {
	started := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := started
	store, state := deviceLimitStore(t, &clock)
	_, claim, err := store.Create([]string{"item"}, 24*time.Hour, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := store.Claim(claim, "First browser")
	if err != nil {
		t.Fatal(err)
	}
	stream := holdStream(t, store, first)
	if status := serveRange(t, store, first, "bytes=3-"); status != http.StatusPartialContent {
		t.Fatalf("first device overlapping range = %d", status)
	}
	clock = started.Add(9 * time.Hour)
	second, _, err := store.Claim(claim, "Second browser")
	if err != nil {
		t.Fatal(err)
	}
	persisted := string(state.data)
	for attempt := range 3 {
		if status := serveRange(t, store, second, "bytes=0-"); status != http.StatusNotFound {
			t.Fatalf("second device attempt %d while first streams = %d", attempt+1, status)
		}
	}
	if string(state.data) != persisted {
		t.Fatal("refused stream changed persisted share state")
	}
	stream.finish()
	for attempt := range 2 {
		if status := serveRange(t, store, second, "bytes=0-"); status != http.StatusPartialContent {
			t.Fatalf("second device attempt %d after first closed = %d", attempt+1, status)
		}
	}
}
