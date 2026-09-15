package mediaprobe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

const testWebVTT = "WEBVTT\n\n00:00.000 --> 00:02.000\nDoor closes\n"

func TestEmbeddedSubtitleExtractionCacheAndTimeline(t *testing.T) { //nolint:cyclop,funlen,gocognit // One lifecycle proves extraction, cache reuse, repair, and timeline mapping.
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	media, calls, ffmpeg := filepath.Join(root, "film.mkv"), filepath.Join(root, "calls"), filepath.Join(root, "ffmpeg")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '"+testWebVTT+"'\n")
	item := library.Item{ID: "film", Path: media}
	probe := embeddedTestProbe(item)

	uncached, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg})
	if err != nil || uncached.Path != "" || string(uncached.Data) != testWebVTT {
		t.Fatalf("uncached subtitle = %#v, %v", uncached, err)
	}
	response := httptest.NewRecorder()
	uncached.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle", nil), time.Time{})
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/vtt; charset=utf-8" || response.Body.String() != testWebVTT {
		t.Fatalf("uncached HTTP = %d %q %#v", response.Code, response.Body.String(), response.Header())
	}
	cached, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache})
	if err != nil || cached.Path == "" || len(cached.Data) != 0 {
		t.Fatalf("cached subtitle = %#v, %v", cached, err)
	}
	if data, readErr := privatefile.Read(cached.Path, embeddedSubtitleLimit); readErr != nil || string(data) != testWebVTT {
		t.Fatalf("cache data = %q, %v", data, readErr)
	}
	response = httptest.NewRecorder()
	cached.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle", nil), time.Time{})
	if response.Code != http.StatusOK || response.Body.String() != testWebVTT {
		t.Fatalf("cached HTTP = %d %q", response.Code, response.Body.String())
	}
	if again, reuseErr := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{CacheDir: cache}); reuseErr != nil || again.Path != cached.Path {
		t.Fatalf("reused subtitle = %#v, %v", again, reuseErr)
	}
	timeline := playback.Timeline{SourceDuration: 2, Duration: 1, Omitted: []playback.Range{{Start: 0, End: 1}}}
	mapped, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{CacheDir: cache, Timeline: &timeline})
	if err != nil || mapped.Path != "" || !mapped.Mapped || !strings.Contains(string(mapped.Data), "00:00:00.000 --> 00:00:01.000") {
		t.Fatalf("mapped subtitle = %#v, %v", mapped, err)
	}
	response = httptest.NewRecorder()
	mapped.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle", nil), time.Now())
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "00:00:00.000 --> 00:00:01.000") || response.Header().Get("Last-Modified") != "" {
		t.Fatalf("mapped HTTP = %d %q %#v", response.Code, response.Body.String(), response.Header())
	}
	zeroTimeline := playback.Timeline{}
	if raw, rawErr := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{CacheDir: cache, Timeline: &zeroTimeline}); rawErr != nil || raw.Path != cached.Path {
		t.Fatalf("unmapped cached subtitle = %#v, %v", raw, rawErr)
	}
	if _, tokenErr := probe.EmbeddedPlayback(t.Context(), item, 3, "invalid", playback.HLSRecipePolicy{}, EmbeddedOptions{CacheDir: cache}); !errors.Is(tokenErr, ErrEmbeddedSubtitleStream) {
		t.Fatalf("invalid playback token error = %v", tokenErr)
	}
	if played, playErr := probe.EmbeddedPlayback(t.Context(), item, 3, "", playback.HLSRecipePolicy{}, EmbeddedOptions{CacheDir: cache}); playErr != nil || played.Path != cached.Path {
		t.Fatalf("playback subtitle = %#v, %v", played, playErr)
	}

	if err = os.WriteFile(cached.Path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	repaired, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache})
	if err != nil || repaired.Path != cached.Path {
		t.Fatalf("repaired subtitle = %#v, %v", repaired, err)
	}
	if data, readErr := os.ReadFile(calls); readErr != nil || string(data) != "xxx" {
		t.Fatalf("FFmpeg calls = %q, %v", data, readErr)
	}
}

func TestEmbeddedSubtitleRejectsUnsupportedStreamsWithoutExtraction(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	calls, ffmpeg := filepath.Join(root, "calls"), filepath.Join(root, "ffmpeg")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '"+testWebVTT+"'\n")
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	probe := New("unused")
	probe.cache[item.ID] = Result{SubtitleFacts: []SubtitleFacts{{SourceIndex: 3, Text: true}, {SourceIndex: 4, Codec: "pgs"}}}
	for _, stream := range []int{-1, 4, 5, 65536} {
		if _, err := probe.Embedded(t.Context(), item, stream, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache}); !errors.Is(err, ErrEmbeddedSubtitleStream) {
			t.Errorf("stream %d error = %v", stream, err)
		}
	}
	if _, err := os.Stat(calls); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported stream ran FFmpeg: %v", err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsupported stream cache = %#v, %v", entries, err)
	}
}

func TestEmbeddedHandlerPreservesPlayerHTTPFailuresAndBytes(t *testing.T) { //nolint:cyclop // One route matrix proves strict parsing, error copy, and exact subtitle bytes.
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	probe := embeddedTestProbe(item)
	options := EmbeddedOptions{}
	handler := probe.EmbeddedHandler(EmbeddedHTTPAdapter{
		Item: func(_ *http.Request, id string) (library.Item, bool) { return item, id == item.ID },
		Options: func() EmbeddedOptions {
			return options
		},
		Error: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			http.Error(writer, message, status)
		},
	}, playback.HLSRecipePolicy{})
	mux := http.NewServeMux()
	mux.Handle("GET /subtitle/{id}/{stream}", handler)
	for name, path := range map[string]string{ //nolint:gosec // These are HTTP test-case paths, not credentials.
		"hidden":      "/subtitle/hidden/3",
		"malformed":   "/subtitle/film/not-a-stream",
		"unsupported": "/subtitle/film/4",
		"token":       "/subtitle/film/3?playbackToken=invalid",
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
			if response.Code != http.StatusNotFound || response.Body.String() != "not found\n" {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/film/3", nil))
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "embedded subtitles are unavailable\n" {
		t.Fatalf("unavailable response = %d %q", response.Code, response.Body.String())
	}
	ffmpeg := filepath.Join(root, "ffmpeg")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf '%s' '"+testWebVTT+"'\n")
	options.FFmpeg = ffmpeg
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/film/3", nil))
	if response.Code != http.StatusOK || response.Body.String() != testWebVTT || response.Header().Get("Content-Type") != "text/vtt; charset=utf-8" {
		t.Fatalf("subtitle response = %d %q %#v", response.Code, response.Body.String(), response.Header())
	}
}

func TestEmbeddedSubtitleBoundsCancellationAndCleanup(t *testing.T) { //nolint:cyclop,funlen,gocognit // The matrix proves all command and cache failure paths without partial files.
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	probe := embeddedTestProbe(item)
	tests := map[string]struct {
		script string
		name   string
		ctx    func() (context.Context, context.CancelFunc)
	}{
		"empty":     {script: "#!/bin/sh\nexit 0\n", name: "empty"},
		"failed":    {script: "#!/bin/sh\nexit 1\n", name: "failed"},
		"oversized": {script: "#!/bin/sh\nhead -c 16777217 /dev/zero\n", name: "oversized"},
		"canceled": {
			script: "#!/bin/sh\nsleep 2\nprintf '%s' '" + testWebVTT + "'\n",
			name:   "canceled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
				return ctx, cancel
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ffmpeg, cache := filepath.Join(root, test.name), filepath.Join(root, test.name+"-cache")
			writeProbeScript(t, ffmpeg, test.script)
			ctx, cancel := t.Context(), func() {}
			if test.ctx != nil {
				ctx, cancel = test.ctx()
			}
			defer cancel()
			if _, err := probe.Embedded(ctx, item, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache}); err == nil {
				t.Fatal("failed extraction succeeded")
			}
			if _, err := os.Stat(embeddedSubtitlePath(cache, item, 3)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed extraction wrote cache: %v", err)
			}
		})
	}
	for name, executable := range map[string]string{"empty": "", "oversized": strings.Repeat("x", 4097), "nul": "ffmpeg\x00"} {
		t.Run("executable-"+name, func(t *testing.T) {
			if _, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: executable}); err == nil || !strings.Contains(err.Error(), "invalid FFmpeg executable") {
				t.Fatalf("executable error = %v", err)
			}
		})
	}

	ffmpeg, cache := filepath.Join(root, "valid"), filepath.Join(root, "write-failure")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf '%s' '"+testWebVTT+"'\n")
	target := embeddedSubtitlePath(cache, item, 3)
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache}); err == nil {
		t.Fatal("cache write failure succeeded")
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(target) {
		t.Fatalf("cache cleanup = %#v, %v", entries, err)
	}
}

func TestEmbeddedSubtitleRepairsUnsafeCacheAndRejectsUnsafeIDs(t *testing.T) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for the repair/rejection matrix.
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	media, ffmpeg := filepath.Join(root, "film.mkv"), filepath.Join(root, "ffmpeg")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf '%s' '"+testWebVTT+"'\n")
	item := library.Item{ID: "film", Path: media}
	probe := embeddedTestProbe(item)
	path := embeddedSubtitlePath(cache, item, 3)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unsafe"), 0o644); err != nil { //nolint:gosec // World-readable mode intentionally simulates an unsafe cache entry.
		t.Fatal(err)
	}
	if result, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache}); err != nil || result.Path != path {
		t.Fatalf("unsafe cache repair = %#v, %v", result, err)
	}
	if data, err := privatefile.Read(path, embeddedSubtitleLimit); err != nil || string(data) != testWebVTT {
		t.Fatalf("repaired cache = %q, %v", data, err)
	}
	for _, id := range []string{"", "../film", strings.Repeat("x", 257)} {
		unsafe := item
		unsafe.ID = id
		probe.cache[id] = probe.cache[item.ID]
		result, err := probe.Embedded(t.Context(), unsafe, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache})
		if err != nil || result.Path != "" || string(result.Data) != testWebVTT {
			t.Errorf("unsafe ID %q = %#v, %v", id, result, err)
		}
	}
}

func embeddedTestProbe(item library.Item) *Probe {
	probe := New("unused")
	probe.cache[item.ID] = Result{SubtitleFacts: []SubtitleFacts{{SourceIndex: 3, Codec: "subrip", Language: "eng", Text: true}}}
	return probe
}
