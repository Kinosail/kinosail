package playback

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTrickplayRoutingAndGenerationEdges(t *testing.T) {
	root := t.TempDir()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	source := filepath.Join(root, "source.mkv")
	writeTestFile(t, source, "source")
	callback := ""
	frames, err := NewTrickplay(TrickplayDependencies{
		Cache: root, FFmpeg: "/missing", RecipePolicy: policy,
		Lookup: func(*http.Request, string) (library.Item, bool) {
			return library.Item{ID: "0123456789abcdef", Kind: "video", Path: source}, true
		},
		NotFound:    func(http.ResponseWriter, *http.Request) { callback = "not-found" },
		Unavailable: func(http.ResponseWriter, *http.Request) { callback = "unavailable" },
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	frames.Register(mux)
	frames.Serve(httptest.NewRecorder(), nil)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/trickplay/0123456789abcdef/10", nil)
	request.SetPathValue("id", "0123456789abcdef")
	request.SetPathValue("second", "10")
	frames.Serve(httptest.NewRecorder(), request)
	if callback != "unavailable" {
		t.Fatalf("generation callback = %q", callback)
	}
	target := filepath.Join(root, "trickplay", "0123456789abcdef", "0.jpg")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, target, "jpeg")
	if err := os.Chtimes(target, time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	token := HLSRecipe{Mode: "transcode", Omitted: []Range{{Start: 10, End: 20}}}.Token()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/trickplay/0123456789abcdef/0?playbackToken="+token, nil)
	request.SetPathValue("id", "0123456789abcdef")
	request.SetPathValue("second", "0")
	response := httptest.NewRecorder()
	frames.Serve(response, request)
	if response.Header().Get("Content-Type") != "image/jpeg" || response.Code != http.StatusOK {
		t.Fatalf("trickplay response = %d, %q", response.Code, response.Header().Get("Content-Type"))
	}
	blockedParent := filepath.Join(root, "blocked")
	writeTestFile(t, blockedParent, "file")
	if err := GenerateTrickplay(t.Context(), root, "/missing", source, filepath.Join(blockedParent, "frame.jpg"), 0); err == nil {
		t.Fatal("ignored trickplay directory failure")
	}
	failedTarget := filepath.Join(root, "failed.jpg")
	if err := GenerateTrickplay(t.Context(), root, "/missing", source, failedTarget, 0); err == nil || pathExists(failedTarget+".tmp.jpg") {
		t.Fatalf("failed generation = %v", err)
	}
}
