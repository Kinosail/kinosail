package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Public completed cold playlists may correct the existing final cut from
// generated packets. Missing/corrupt metadata must never invent media or URIs.
func copiedColdPlaylist(t *testing.T) (copiedHTTPFixture, string) {
	t.Helper()
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	if err := os.Remove(filepath.Join(f.directory, ".copy-timeline")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.directory, "360p", "index.m3u8")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	index := strings.LastIndex(text, "#EXTINF:4,")
	if index < 0 {
		t.Fatal("fixture final cut missing")
	}
	text = text[:index] + strings.Replace(text[index:], "#EXTINF:4,", "#EXTINF:3.9,", 1)
	if err := writeCopiedCacheFile(f.config.CacheDir, path, []byte(text)); err != nil {
		t.Fatal(err)
	}
	return f, strings.TrimSuffix(f.source, "index.m3u8") + "360p/index.m3u8"
}

func TestCopiedHLSEOFHTTPUsesMeasuredEndpointAndInvalidatesChangedAssets(t *testing.T) {
	f, path := copiedColdPlaylist(t)
	first := apiCall(t, f.handler, "", http.MethodGet, path, nil)
	assertAPIBody(t, first, http.StatusOK, "#EXTINF:4.000000,", "#EXT-X-PLAYLIST-TYPE:VOD", "#EXT-X-ENDLIST")
	if strings.Count(first.Body.String(), "#EXTINF:") != 3 {
		t.Fatal("endpoint invented a cut")
	}
	before, err := os.ReadFile(f.probes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(before), "endpoint") != 1 {
		t.Fatal("endpoint was not measured")
	}
	assertAPIBody(t, apiCall(t, f.handler, "", http.MethodGet, path, nil), http.StatusOK, "#EXTINF:4.000000,")
	unchanged, err := os.ReadFile(f.probes)
	if err != nil || string(unchanged) != string(before) {
		t.Fatal("unchanged completed media was repeatedly probed")
	}
	if err := os.WriteFile(f.endpoint, []byte(`{"packets":[{"pts_time":"12.023","duration_time":"0.04"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.directory, "360p", "segment-00002.m4s"), []byte("changed final fragment identity"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertAPIBody(t, apiCall(t, f.handler, "", http.MethodGet, path, nil), http.StatusOK, "#EXTINF:3.980000,")
	after, err := os.ReadFile(f.probes)
	if err != nil || strings.Count(string(after), "endpoint") != 2 {
		t.Fatal("changed generated media reused stale endpoint")
	}
}

func TestCopiedHLSEOFHTTPRejectsUnsafeProbeAndAssetInputs(t *testing.T) {
	tests := []copiedEndpointCase{
		{"missing clock", `{}`, "", ""},
		{"non-key first packet", `{"packets":[{"pts_time":"0","flags":"__"}]}`, "", ""},
		{"negative clock", `{"packets":[{"pts_time":"-1","flags":"K"}]}`, "", ""},
		{"clock outside mux window", `{"packets":[{"pts_time":"1.1","flags":"K"}]}`, "", ""},
		{"nonfinite clock", `{"packets":[{"pts_time":"NaN","flags":"K"}]}`, "", ""},
		{"oversized clock", strings.Repeat("x", 8193), "", ""},
		{"missing endpoint", "", `{}`, ""},
		{"malformed endpoint", "", `not-json`, ""},
		{"invalid duration", "", `{"packets":[{"pts_time":"12","duration_time":"0"}]}`, ""},
		{"nonfinite endpoint", "", `{"packets":[{"pts_time":"Infinity","duration_time":"1"}]}`, ""},
		{"wrong endpoint", "", `{"packets":[{"pts_time":"50","duration_time":"1"}]}`, ""},
		{"oversized endpoint", "", strings.Repeat("x", (1<<20)+1), ""},
		{"empty last fragment", "", "", "empty"},
		{"symlink last fragment", "", "", "symlink"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, path, outside := prepareCopiedEndpointCase(t, test)
			assertCopiedEndpointRejected(t, f, path, outside)
		})
	}
}

type copiedEndpointCase struct{ name, clock, endpoint, asset string }

func prepareCopiedEndpointCase(t *testing.T, test copiedEndpointCase) (copiedHTTPFixture, string, string) {
	t.Helper()
	f, path := copiedColdPlaylist(t)
	for file, value := range map[string]string{f.clock: test.clock, f.endpoint: test.endpoint} {
		if value != "" {
			if err := os.WriteFile(file, []byte(value), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	final := filepath.Join(f.directory, "360p", "segment-00002.m4s")
	outside := filepath.Join(t.TempDir(), "protected")
	if err := os.WriteFile(outside, []byte("do not modify"), 0o600); err != nil {
		t.Fatal(err)
	}
	if test.asset == "empty" {
		if err := os.WriteFile(final, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if test.asset == "symlink" {
		if err := os.Remove(final); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, final); err != nil {
			t.Fatal(err)
		}
	}

	return f, path, outside
}

func assertCopiedEndpointRejected(t *testing.T, f copiedHTTPFixture, path, outside string) {
	t.Helper()
	before, err := os.ReadFile(f.starts)
	if err != nil {
		t.Fatal(err)
	}
	response := apiCall(t, f.handler, "", http.MethodGet, path, nil)
	assertAPIBody(t, response, http.StatusOK, "#EXTINF:3.9,", "#EXT-X-ENDLIST")
	if strings.Count(response.Body.String(), "#EXTINF:") != 3 || strings.Contains(response.Body.String(), "segment-00003") {
		t.Fatal("unsafe EOF data invented media")
	}
	after, err := os.ReadFile(f.starts)
	if err != nil || string(after) != string(before) {
		t.Fatal("invalid endpoint triggered encoding")
	}
	protected, err := os.ReadFile(outside)
	if err != nil || string(protected) != "do not modify" {
		t.Fatal("invalid cache asset modified unrelated file")
	}
}

// Bound cache writes to the owned disposable root even when HTTP item identity
// determines the selected cache subdirectory.
func writeCopiedCacheFile(cache, path string, data []byte) error {
	root, err := os.OpenRoot(cache)
	if err != nil {
		return err
	}
	defer root.Close()
	name, err := filepath.Rel(cache, path)
	if err != nil {
		return err
	}
	return root.WriteFile(name, data, 0o600)
}

// A valid cached rendition can be temporarily unpublished while its producer
// finishes a fragment. Missing index metadata must not turn that wait into 404.
func TestCopiedHLSHTTPWaitsForValidMediaDuringRenditionPublication(t *testing.T) {
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	rendition := filepath.Join(f.directory, "360p")
	index, err := os.ReadFile(filepath.Join(rendition, "index.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.m3u8", "segment-00002.m4s"} {
		if err := os.Remove(filepath.Join(rendition, name)); err != nil {
			t.Fatal(err)
		}
	}
	published := make(chan error, 1)
	go func() {
		time.Sleep(30 * time.Millisecond)
		if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(rendition, "index.m3u8"), index); err != nil {
			published <- err
			return
		}
		published <- writeCopiedCacheFile(f.config.CacheDir, filepath.Join(rendition, "segment-00002.m4s"), []byte("published fragment"))
	}()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(f.source, "index.m3u8")+"360p/segment-00002.m4s", nil))
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	assertAPIBody(t, response, http.StatusOK, "published fragment")
}
