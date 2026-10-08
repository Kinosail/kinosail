package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fresh corrected public fixtures do not schedule malformed retained manifests.
// These isolated cases protect admission before the cached-file shortcut.
func TestRemainingAACManifestCannotBeBypassed(t *testing.T) {
	for _, invalid := range []string{"missing", "malformed", "oversized", "symlink", "undeclared", "duplicate", "bad-duration", "nonzero-sequence"} {
		t.Run(invalid, func(t *testing.T) {
			manager, item, recipe, directory := remainingAACAdmissionFixture(t)
			path := filepath.Join(directory, "audio/index.m3u8")
			asset := "audio/segment-00000.m4s"
			switch invalid {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(t.TempDir(), "manifest")
				writeHLSLoadingFile(t, target, remainingInitialAACPrefix+"#EXT-X-ENDLIST\n")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				writeHLSLoadingFile(t, path, strings.Repeat("#", (1<<20)+1))
			case "undeclared":
				asset = "audio/segment-99999.m4s"
				writeHLSLoadingFile(t, filepath.Join(directory, asset), "unadvertised-cached-fragment")
			case "duplicate":
				writeHLSLoadingFile(t, path, strings.ReplaceAll(remainingInitialAACPrefix, "segment-00001", "segment-00000"))
			case "bad-duration":
				writeHLSLoadingFile(t, path, strings.ReplaceAll(remainingInitialAACPrefix, "2.005333", "NaN"))
			case "nonzero-sequence":
				writeHLSLoadingFile(t, path, strings.ReplaceAll(remainingInitialAACPrefix, "MEDIA-SEQUENCE:0", "MEDIA-SEQUENCE:99999"))
			default:
				writeHLSLoadingFile(t, path, "#EXTM3U\n#EXT-X-ENDLIST\n")
			}
			for _, name := range []string{asset, "audio/init.mp4"} {
				if invalid == "undeclared" && name == "audio/init.mp4" {
					continue
				}
				response := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/fixture/"+name, nil)
				manager.serveRecipe(response, request, item, recipe, name)
				if response.Code != http.StatusNotFound || len(manager.jobs) != 0 {
					t.Fatalf("invalid %s manifest admitted%s: status%d jobs%d", invalid, name, response.Code, len(manager.jobs))
				}
			}
		})
	}
}

func TestRemainingAACProjectedCachedFragmentsRemainReadable(t *testing.T) {
	manager, item, recipe, directory := remainingAACAdmissionFixture(t)
	prefix := strings.Split(remainingInitialAACPrefix, "#EXTINF:2.005333,\nsegment-00004.m4s")[0]
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), prefix)
	for _, name := range []string{"audio/segment-00004.m4s", "audio/segment-00005.m4s"} {
		writeHLSLoadingFile(t, filepath.Join(directory, name), "projected-current-fragment")
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/fixture/"+name, nil)
		manager.serveRecipe(response, request, item, recipe, name)
		if response.Code != http.StatusOK || response.Body.String() != "projected-current-fragment" || len(manager.jobs) != 0 {
			t.Fatalf("valid projected cached%s rejected: status%d jobs%d", name, response.Code, len(manager.jobs))
		}
	}
}
