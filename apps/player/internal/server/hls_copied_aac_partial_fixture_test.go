package server

import (
	"crypto/sha256"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

// These scheduling fixtures protect read-only partial admission; actual Server
// HEAD/GET compatibility remains the separately retained hosted public journey.
func copiedAACPartialFixture(t *testing.T, shape string) (*hlsManager, library.Item, hlsRecipe, string) {
	t.Helper()
	manager, item, recipe, directory, _ := copiedAACLegacyFixture(t)
	if shape != "complete" {
		if err := os.Remove(filepath.Join(directory, "360p/segment-00001.m4s")); err != nil {
			t.Fatal(err)
		}
	}
	if shape == "prefix" || shape == "speculative" {
		prefix := strings.Replace(copiedRecoveryManifest, "#EXTINF:2.000000,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n", "", 1)
		writeHLSLoadingFile(t, filepath.Join(directory, "360p/index.m3u8"), prefix)
	}
	if shape == "speculative" {
		writeHLSLoadingFile(t, filepath.Join(directory, ".startup"), "1")
	}
	return manager, item, recipe, directory
}

func copiedAACPartialSnapshot(t *testing.T, directory string) map[string]copiedAACLegacyEntry {
	t.Helper()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	result := map[string]copiedAACLegacyEntry{}
	err = fs.WalkDir(root.FS(), ".", func(name string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if len(result) >= 128 {
			t.Fatal("partial read inventory exceeded fixture bound")
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		entry := copiedAACLegacyEntry{info: info}
		if info.Mode().IsRegular() {
			data, err := copiedAACLegacyInventoryContent(root, name, info)
			if err != nil {
				return err
			}
			entry.hash = sha256.Sum256(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := root.Readlink(name)
			if err != nil {
				return err
			}
			entry.hash = sha256.Sum256([]byte(target))
		}
		result[name] = entry
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func copiedAACPartialResponse(t *testing.T, manager *hlsManager, item library.Item, recipe hlsRecipe, name, method, rangeValue string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "/"+name, nil)
	if rangeValue != "" {
		request.Header.Set("Range", rangeValue)
	}
	writer := httptest.NewRecorder()
	if !manager.serveCopiedHLSLegacy(writer, request, item, recipe, name) {
		t.Error("present partial Version1 binding fell through to preparation")
	}
	return writer
}

func copiedAACPartialSource(t *testing.T, path string) copiedAACLegacyEntry {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		t.Fatal("partial source fixture invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil || int64(len(data)) != info.Size() {
		t.Fatal("partial source fixture read failed")
	}
	return copiedAACLegacyEntry{info: info, hash: sha256.Sum256(data)}
}

func requireCopiedAACPartialSource(t *testing.T, before copiedAACLegacyEntry, path string) {
	t.Helper()
	after := copiedAACPartialSource(t, path)
	requireCopiedAACLegacyInventory(t, map[string]copiedAACLegacyEntry{"source": before}, map[string]copiedAACLegacyEntry{"source": after})
}

func copiedAACPartialBaseline(t *testing.T, manager *hlsManager, item library.Item, recipe hlsRecipe, name, method, rangeValue string) *httptest.ResponseRecorder {
	t.Helper()
	result := copiedAACPartialResponse(t, manager, item, recipe, name, method, rangeValue)
	expected := http.StatusOK
	if rangeValue != "" {
		expected = http.StatusPartialContent
	}
	if result.Code != expected || method == http.MethodHead && result.Body.Len() != 0 {
		t.Fatal("otherwise-valid complete Version1 control did not qualify")
	}
	return result
}
