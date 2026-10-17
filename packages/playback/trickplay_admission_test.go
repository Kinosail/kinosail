package playback

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type trickplayFixture struct {
	root, runs                 string
	mux                        *http.ServeMux
	acquired, held, requesters atomic.Int64
}

func newTrickplayFixture(t *testing.T, encoder string) *trickplayFixture {
	t.Helper()
	fixture := &trickplayFixture{root: t.TempDir(), mux: http.NewServeMux()}
	source := filepath.Join(fixture.root, "source.mkv")
	writeTestFile(t, source, "source")
	fixture.runs = filepath.Join(fixture.root, "runs.log")
	ffmpeg := linkFakeEncoder(t, fixture.root, encoder)
	acquire := func(ctx context.Context) (func(), error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fixture.acquired.Add(1)
		fixture.held.Add(1)
		return func() { fixture.held.Add(-1) }, nil
	}
	lookup := func(_ *http.Request, id string) (library.Item, bool) {
		fixture.requesters.Add(1)
		return library.Item{ID: id, Kind: "video", Path: source}, true
	}
	failure := func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(writer, message, status)
	}
	MustTrickplayRegistration(fixture.root, ffmpeg, HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}, lookup, acquire, failure)(fixture.mux)
	return fixture
}

func (fixture *trickplayFixture) serve(ctx context.Context, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	fixture.mux.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))
	return response
}

func TestTrickplayAdmissionGatesAndCollapsesConcurrentGeneration(t *testing.T) {
	t.Parallel()
	fixture := newTrickplayFixture(t, "slow-frame")
	root, runs, mux := fixture.root, fixture.runs, fixture.mux
	acquired, held := &fixture.acquired, &fixture.held

	if codes := concurrentTrickplayCodes(t, mux, "/trickplay/video/27", 6); codes != [6]int{200, 200, 200, 200, 200, 200} {
		t.Fatalf("concurrent response codes = %v", codes)
	}
	if count := len(readTestFile(t, runs)); count != 1 || acquired.Load() != 1 || held.Load() != 0 {
		t.Fatalf("ffmpeg runs = %d, acquisitions = %d, held = %d", count, acquired.Load(), held.Load())
	}
	if len(scratchFiles(t, filepath.Join(root, "trickplay", "video"))) != 0 {
		t.Fatal("scratch frame remained after generation")
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	response := fixture.serve(cancelled, "/trickplay/video/47")
	if response.Code != http.StatusServiceUnavailable || len(readTestFile(t, runs)) != 1 || pathExists(filepath.Join(root, "trickplay", "video", "40.jpg")) {
		t.Fatalf("rejected admission = %d, runs = %d", response.Code, len(readTestFile(t, runs)))
	}
}

func TestTrickplayGenerationOutlivesADisconnectedRequester(t *testing.T) {
	t.Parallel()
	fixture := newTrickplayFixture(t, "gated-frame")
	leader, disconnect := context.WithCancel(t.Context())
	defer disconnect()
	leaderCode := make(chan int, 1)
	go func() { leaderCode <- fixture.serve(leader, "/trickplay/video/27").Code }()
	waitForTrickplay(t, func() bool { return len(readTestFile(t, fixture.runs)) == 1 })
	waiter := make(chan *httptest.ResponseRecorder, 1)
	go func() { waiter <- fixture.serve(t.Context(), "/trickplay/video/23") }()
	waitForTrickplay(t, func() bool { return fixture.requesters.Load() == 2 })
	disconnect()
	if code := <-leaderCode; code != http.StatusServiceUnavailable {
		t.Fatalf("disconnected requester = %d", code)
	}
	writeTestFile(t, filepath.Join(fixture.root, "release"), "")
	response := <-waiter
	if response.Code != http.StatusOK || response.Body.String() != "jpeg" {
		t.Fatalf("joined requester = %d %q", response.Code, response.Body.String())
	}
	if count := len(readTestFile(t, fixture.runs)); count != 1 || fixture.acquired.Load() != 1 || fixture.held.Load() != 0 {
		t.Fatalf("ffmpeg runs = %d, acquisitions = %d, held = %d", count, fixture.acquired.Load(), fixture.held.Load())
	}
}

func TestTrickplayEncoderMissesAreNeitherServedNorRepeated(t *testing.T) {
	t.Parallel()
	for _, encoder := range []string{"empty-frame", "failed-frame"} {
		t.Run(encoder, func(t *testing.T) {
			t.Parallel()
			fixture := newTrickplayFixture(t, encoder)
			for attempt := range 2 {
				if response := fixture.serve(t.Context(), "/trickplay/video/27"); response.Code != http.StatusNotFound {
					t.Fatalf("attempt %d = %d %q", attempt, response.Code, response.Body.String())
				}
			}
			frames := filepath.Join(fixture.root, "trickplay", "video")
			if count := len(readTestFile(t, fixture.runs)); count != 1 || fixture.held.Load() != 0 {
				t.Fatalf("ffmpeg runs = %d, held = %d", count, fixture.held.Load())
			}
			if pathExists(filepath.Join(frames, "20.jpg")) || len(scratchFiles(t, frames)) != 0 {
				t.Fatal("encoder miss left a frame in the cache")
			}
		})
	}
}

func waitForTrickplay(t *testing.T, condition func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if condition() {
			return
		}
	}
	t.Fatal("trickplay generation never reached the expected state")
}

func concurrentTrickplayCodes(t *testing.T, handler http.Handler, path string, count int) [6]int {
	t.Helper()
	var wait sync.WaitGroup
	var codes [6]int
	for index := range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
			codes[index] = response.Code
		}()
	}
	wait.Wait()
	return codes
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // Test reads its own temporary file.
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return content
}
