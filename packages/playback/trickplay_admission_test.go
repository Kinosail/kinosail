package playback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTrickplayAdmissionGatesAndCollapsesConcurrentGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "source.mkv")
	writeTestFile(t, source, "source")
	runs := filepath.Join(root, "runs.log")
	ffmpeg := writeFakeFFmpeg(t, root, runs)
	var acquired, held atomic.Int64
	acquire := func(ctx context.Context) (func(), error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		acquired.Add(1)
		held.Add(1)
		return func() { held.Add(-1) }, nil
	}
	lookup := func(_ *http.Request, id string) (library.Item, bool) {
		return library.Item{ID: id, Kind: "video", Path: source}, true
	}
	failure := func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(writer, message, status)
	}
	mux := http.NewServeMux()
	MustTrickplayRegistration(root, ffmpeg, HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}, lookup, acquire, failure)(mux)

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
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(cancelled, http.MethodGet, "/trickplay/video/47", nil))
	if response.Code != http.StatusServiceUnavailable || len(readTestFile(t, runs)) != 1 || pathExists(filepath.Join(root, "trickplay", "video", "40.jpg")) {
		t.Fatalf("rejected admission = %d, runs = %d", response.Code, len(readTestFile(t, runs)))
	}
}

func writeFakeFFmpeg(t *testing.T, root, runs string) string {
	t.Helper()
	ffmpeg := filepath.Join(root, "ffmpeg.sh")
	script := fmt.Sprintf("#!/bin/sh\nprintf x >> %q\nsleep 0.2\nfor argument; do last=\"$argument\"; done\nprintf jpeg > \"$last\"\n", runs)
	if err := os.WriteFile(ffmpeg, []byte(script), 0o700); err != nil { //nolint:gosec // The fake encoder must be executable by the test process.
		t.Fatal(err)
	}
	return ffmpeg
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
