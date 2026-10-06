package server_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// A completed copied stream can begin at an earlier IDR than its requested
// offset. Its measured generated window owns admission of the final cut; an
// untrusted manifest or failed metadata probe cannot extend that window.
func TestCopiedHLSHTTPCompletedPrerollWindowAdmitsOnlyMeasuredMedia(t *testing.T) {
	for _, value := range []struct {
		name, endpoint, asset string
		status                int
	}{
		{"measured earlier IDR", "", "", http.StatusOK},
		{"missing endpoint", `{}`, "", http.StatusNotFound},
		{"malformed endpoint", `not-json`, "", http.StatusNotFound},
		{"endpoint beyond source", `{"packets":[{"pts_time":"20","duration_time":"0.04"}]}`, "", http.StatusNotFound},
		{"nonfinite endpoint", `{"packets":[{"pts_time":"Infinity","duration_time":"0.04"}]}`, "", http.StatusNotFound},
		{"truncated initialization", "", "init.mp4", http.StatusNotFound},
		{"missing final fragment", "", "segment-00002.m4s", http.StatusNotFound},
		{"unknown cached ordinal", "", "segment-99999.m4s", http.StatusNotFound},
	} {
		t.Run(value.name, func(t *testing.T) { assertCompletedPrerollAdmission(t, value.endpoint, value.asset, value.status) })
	}
}

func assertCompletedPrerollAdmission(t *testing.T, endpoint, asset string, status int) {
	t.Helper()
	f := copiedPrerollWindow(t)
	name := damageCopiedPrerollWindow(t, f, endpoint, asset)
	before := snapshotCopiedPolicyCache(t, f, status != http.StatusOK)
	restarted := server.New(f.config)
	response := apiCall(t, restarted, "", http.MethodGet, strings.TrimSuffix(f.source, "index.m3u8")+"360p/"+name, nil)
	assertAPIBody(t, response, status)
	if status == http.StatusOK {
		assertAPIBody(t, response, http.StatusOK, "fragment-2")
	}
	if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, status != http.StatusOK)) {
		t.Fatal("completed window admission mutated cache or started encoding")
	}
}

func damageCopiedPrerollWindow(t *testing.T, f copiedHTTPFixture, endpoint, asset string) string {
	t.Helper()
	if endpoint != "" {
		if err := os.WriteFile(f.endpoint, []byte(endpoint), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	name := "segment-00002.m4s"
	switch asset {
	case "init.mp4":
		if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, "360p", asset), []byte("truncated")); err != nil {
			t.Fatal(err)
		}
	case "segment-00002.m4s":
		if err := os.Remove(filepath.Join(f.directory, "360p", asset)); err != nil {
			t.Fatal(err)
		}
	case "segment-99999.m4s":
		name = asset
		if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, "360p", name), []byte("unlisted")); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

func copiedPrerollWindow(t *testing.T) copiedHTTPFixture {
	t.Helper()
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	if err := os.Remove(filepath.Join(f.directory, ".copy-timeline")); err != nil {
		t.Fatal(err)
	}
	old := "r-a0-s0-none-t0-b0"
	token := old + "-o4000"
	for _, name := range []string{".source", "index.m3u8"} {
		path := filepath.Join(f.directory, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeCopiedCacheFile(f.config.CacheDir, path, bytes.ReplaceAll(data, []byte(old), []byte(token))); err != nil {
			t.Fatal(err)
		}
	}
	directory := filepath.Join(f.config.CacheDir, playback.HLSRecipeKey(f.id, playback.HLSRecipe{Mode: "remux", Offset: 4}))
	if err := os.Rename(f.directory, directory); err != nil {
		t.Fatal(err)
	}
	f.directory = directory
	f.source = strings.Replace(f.source, old, token, 1)
	return f
}
