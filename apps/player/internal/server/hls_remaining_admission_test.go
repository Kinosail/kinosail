package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

// Corrected-media cold reopens do not test malformed or substituted cache
// metadata. These isolated requests protect the new cached-asset boundary;
// native AAC content and lifecycle remain hosted public-media admissions.
func TestRemainingCurrentAACInvalidCacheCannotDeliverAssets(t *testing.T) {
	for _, invalid := range []string{"malformed-init", "missing-source", "changed-source", "changed-master", "symlink-init", "symlink-fragment"} {
		for _, asset := range []string{"audio/init.mp4", "audio/segment-00000.m4s"} {
			t.Run(invalid+"/"+asset, func(t *testing.T) {
				remainingAACAssertInvalidCache(t, invalid, asset)
			})
		}
	}
}

func remainingAACAdmissionFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe, string) {
	t.Helper()
	manager, item, recipe := remainingAACCacheFixture(t)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#KINOSAIL-TRANSCODER:"+options.Cache+"\n#EXT-X-STREAM-INF:BANDWIDTH=192000\naudio/index.m3u8\n")
	writeHLSLoadingFile(t, filepath.Join(directory, ".source"), options.Cache)
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), remainingInitialAACPrefix+"#EXT-X-ENDLIST\n")
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/init.mp4"), string(remainingAACCacheInitialization(t)))
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/segment-00000.m4s"), "mock-current-aac-fragment")
	return manager, item, recipe, directory
}

func remainingAACInvalidateFixture(t *testing.T, directory, invalid string) {
	t.Helper()
	switch invalid {
	case "malformed-init":
		writeHLSLoadingFile(t, filepath.Join(directory, "audio/init.mp4"), "invalid-initialization")
	case "missing-source":
		if err := os.Remove(filepath.Join(directory, ".source")); err != nil {
			t.Fatal(err)
		}
	case "changed-source":
		writeHLSLoadingFile(t, filepath.Join(directory, ".source"), "different-source-policy")
	case "changed-master":
		writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#KINOSAIL-TRANSCODER:different-policy\n#EXT-X-STREAM-INF:BANDWIDTH=192000\naudio/index.m3u8\n")
	default:
		name := "audio/init.mp4"
		if invalid == "symlink-fragment" {
			name = "audio/segment-00000.m4s"
		}
		remainingAACSymlinkFixture(t, filepath.Join(directory, name))
	}
}

func remainingAACAssertInvalidCache(t *testing.T, invalid, asset string) {
	t.Helper()
	manager, item, recipe, directory := remainingAACAdmissionFixture(t)
	remainingAACInvalidateFixture(t, directory, invalid)
	path := filepath.Join(directory, asset)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/fixture/"+asset, nil)
	manager.serveRecipe(response, request, item, recipe, asset)
	if response.Code == http.StatusOK && bytes.Equal(response.Body.Bytes(), payload) {
		t.Fatalf("invalid %s cache delivered %s", invalid, asset)
	}
	after, err := os.Lstat(path)
	manager.mu.Lock()
	jobs := len(manager.jobs)
	manager.mu.Unlock()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || jobs != 0 {
		t.Fatal("rejected cached request mutated its asset or scheduled encoding")
	}
}

func remainingAACSymlinkFixture(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "substituted-asset")
	writeHLSLoadingFile(t, target, string(data))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
