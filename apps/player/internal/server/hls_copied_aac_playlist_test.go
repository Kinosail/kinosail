package server

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Gap: public HTTP cannot deterministically replace an opened generation between proof and rendering.
func TestCopiedAACPlaylistUsesRetainedGeneration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	_, directory, held := copiedAACGenerationFixture(t)
	defer held.close()
	for _, name := range []string{"index.m3u8", "360p/index.m3u8"} {
		data, err := held.playlist(name, 0, 20, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8", nil))
		if err != nil || !strings.HasPrefix(string(data), "#EXTM3U\n") {
			t.Fatal("valid retained playlist was not rendered")
		}
	}
	retired := directory + "-playlist-retired"
	if err := os.Rename(directory, retired); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(directory, os.DirFS(retired)); err != nil {
		t.Fatal(err)
	}
	if _, err := held.playlist("index.m3u8", 0, 20, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8", nil)); err == nil {
		t.Fatal("replacement canonical generation inherited retained playlist admission")
	}
	held.close()
}

func TestCopiedAACPlaylistRejectsMissingBindingWithoutMutation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, directory, held := copiedAACGenerationFixture(t)
	item, recipe := held.item, held.recipe
	held.close()
	before := copiedAACPlaylistSnapshot(t, directory)
	if err := os.Rename(filepath.Join(directory, ".source"), filepath.Join(directory, ".source-hidden")); err != nil {
		t.Fatal(err)
	}
	missing := copiedAACPlaylistSnapshot(t, directory)
	for _, name := range []string{"index.m3u8", "360p/index.m3u8", "720p/index.m3u8"} {
		writer := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+name, nil)
		if !manager.serveCopiedAACPlaylist(writer, request, item, recipe, name, filepath.Base(directory), 0, 20) || writer.Code != http.StatusNotFound {
			t.Fatalf("missing binding published %s", name)
		}
		if !reflect.DeepEqual(copiedAACPlaylistSnapshot(t, directory), missing) {
			t.Fatal("rejected playlist mutated cache")
		}
	}
	if err := os.Rename(filepath.Join(directory, ".source-hidden"), filepath.Join(directory, ".source")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(copiedAACPlaylistSnapshot(t, directory), before) {
		t.Fatal("rejected playlist changed restored binding")
	}
}

// Gap: the public transport cannot hold one exact post-admission write to prove lease release.
func TestCopiedAACPlaylistSlowTransferReleasesMetadataLease(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, directory, held := copiedAACGenerationFixture(t)
	item, recipe := held.item, held.recipe
	held.close()
	owner := newCopiedAACDelivery(t)
	request := httptest.NewRequestWithContext(owner.ctx, http.MethodGet, "/index.m3u8", nil)
	go func() {
		owner.done <- manager.serveCopiedAACPlaylist(owner.writer, request, item, recipe, "index.m3u8", filepath.Base(directory), 0, 20)
	}()
	owner.waitEntered(t, "playlist did not reach retained-byte delivery")
	admission, cancelAdmission := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancelAdmission()
	_, releaseAdmission, err := manager.copiedHLSMetadataAdmission(admission)
	if err != nil {
		t.Fatal("slow playlist retained metadata admission")
	}
	releaseAdmission()
	owner.release()
	owner.join(t, "retained playlist delivery failed")
}

func copiedAACPlaylistSnapshot(t *testing.T, directory string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	for _, name := range []string{".source", ".copy-timeline", ".copy-clock", "index.m3u8", "360p/index.m3u8", "360p/init.mp4", "360p/segment-00000.m4s", "360p/segment-00001.m4s"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		result[name] = sha256.Sum256(data)
	}
	return result
}

func TestCopiedAACPlaylistRejectsUnboundMasterURISet(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, directory, held := copiedAACGenerationFixture(t)
	item, recipe, policy := held.item, held.recipe, held.policy
	held.close()
	original, err := os.ReadFile(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	masters := []string{string(original) + "../escape.m3u8\n", string(original) + "360p/index.m3u8\n", strings.Replace(string(original), policy, policy+"-wrong", 1), string(original) + "720p/index.m3u8\n"}
	for _, directive := range []string{
		"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"x\",URI=\"../escape.m3u8\"",
		"#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=1,URI=\"/escape.m3u8\"",
		"#EXT-X-SESSION-KEY:METHOD=AES-128,URI=\"https://example.invalid/key\"",
		"#EXT-X-KEY:METHOD=AES-128,URI=\"360p/index.m3u8\"",
	} {
		masters = append(masters, string(original)+directive+"\n")
	}
	for _, master := range masters {
		writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), master)
		before := copiedAACPlaylistSnapshot(t, directory)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8", nil)
		if !manager.serveCopiedAACPlaylist(response, request, item, recipe, "index.m3u8", filepath.Base(directory), 0, 20) || response.Code != http.StatusNotFound {
			t.Fatal("unbound master URI set was served")
		}
		if !reflect.DeepEqual(copiedAACPlaylistSnapshot(t, directory), before) {
			t.Fatal("rejected master changed cache")
		}
	}
}
