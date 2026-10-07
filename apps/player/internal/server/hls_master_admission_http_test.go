package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// Persisted masters can contain repeated or excessive rendition references.
// Validate the complete bounded URI set before reading initialization files or
// admitting recovery work. Missing/corrupt init recovery has a separate owner.
func TestCopiedHLSHTTPRejectsMalformedMasterBeforeRecoveryEffects(t *testing.T) {
	for _, value := range []struct{ name, master string }{
		{"duplicate", "360p/index.m3u8\n360p/index.m3u8\n"},
		{"six renditions", masterRenditions([]string{"360p", "432p", "540p", "720p", "1080p", "2160p"})},
		{"missing rendition", ""},
		{"unknown asset", "360p/unknown.m3u8\n"},
		{"malformed path", "../360p/index.m3u8\n"},
		{"oversized", strings.Repeat("#padding\n", (1<<20)/9+1)},
	} {
		t.Run(value.name, func(t *testing.T) {
			assertRejectedCopiedMaster(t, value.master)
		})
	}
}

func assertRejectedCopiedMaster(t *testing.T, invalidMaster string) {
	t.Helper()
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	master := copiedMasterMetadata(t, f) + invalidMaster
	if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, "index.m3u8"), []byte(master)); err != nil {
		t.Fatal(err)
	}
	before := snapshotCopiedPolicyCache(t, f, true)
	source, err := os.ReadFile(filepath.Join(f.config.MediaDir, "Copied.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	response := apiCall(t, server.New(f.config), "", http.MethodGet, f.source, nil)
	assertAPIBody(t, response, http.StatusNotFound)
	assertRejectedMasterWarning(t, f.output.snapshot(), response.Header().Get("X-Request-ID"))
	if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, true)) {
		t.Fatal("rejected master changed cache or admitted encoding")
	}
	preparation := prepareSource(t, server.New(f.config), f.id, f.source)
	assertAPIBody(t, preparation, http.StatusNotFound)
	if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, true)) {
		t.Fatal("rejected preparation changed cache or admitted encoding")
	}
	actual, err := os.ReadFile(filepath.Join(f.config.MediaDir, "Copied.mp4"))
	if err != nil || !bytes.Equal(source, actual) {
		t.Fatal("rejected master changed original media")
	}
}

func assertRejectedMasterWarning(t *testing.T, logs, requestID string) {
	t.Helper()
	matches := 0
	for _, line := range strings.Split(logs, "\n") {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil || entry["msg"] != "HLS master rejected" {
			continue
		}
		if entry["level"] != "WARN" || entry["failure_class"] != "invalid-master" || entry["request_id"] != requestID {
			t.Fatal("rejected master lacks bounded, correlated warning")
		}
		matches++
	}
	if matches != 1 {
		t.Fatal("rejected master must emit one correlated warning")
	}
}

func TestCopiedHLSHTTPAcceptsMaximumUniqueMasterWithoutEncoding(t *testing.T) {
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	// Legacy unindexed caches allow the full bounded rendition set. An indexed
	// copy instead binds exactly one rendition to its measured clock certificate.
	for _, name := range []string{".copy-timeline", ".copy-clock"} {
		if err := os.Remove(filepath.Join(f.directory, name)); err != nil {
			t.Fatal(err)
		}
	}
	qualities := []string{"360p", "432p", "540p", "720p", "1080p"}
	populateCopiedMasterRenditions(t, f, qualities)
	master := copiedMasterMetadata(t, f) + masterRenditions(qualities)
	if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, "index.m3u8"), []byte(master)); err != nil {
		t.Fatal(err)
	}
	before := snapshotCopiedPolicyCache(t, f, false)
	response := apiCall(t, server.New(f.config), "", http.MethodGet, f.source, nil)
	assertAPIBody(t, response, http.StatusOK)
	for _, quality := range qualities {
		if strings.Count(response.Body.String(), quality+"/index.m3u8") != 1 {
			t.Fatal("valid bounded master did not preserve every unique rendition")
		}
	}
	if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, false)) {
		t.Fatal("valid bounded master rebuilt cache or admitted encoding")
	}
}

func TestCopiedHLSHTTPCertifiedMasterRejectsAmbiguousOwnerWithoutMutation(t *testing.T) {
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	qualities := []string{"360p", "432p"}
	populateCopiedMasterRenditions(t, f, qualities)
	master := copiedMasterMetadata(t, f) + masterRenditions(qualities)
	if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, "index.m3u8"), []byte(master)); err != nil {
		t.Fatal(err)
	}
	before := snapshotCopiedPolicyCache(t, f, true)
	path := strings.TrimSuffix(f.source, "index.m3u8") + "360p/segment-00000.m4s"
	response := apiCall(t, server.New(f.config), "", http.MethodGet, path, nil)
	assertAPIBody(t, response, http.StatusNotFound)
	if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, true)) {
		t.Fatal("ambiguous certified master changed cache or admitted encoding")
	}
	assertCopiedRecoverySourceUnchanged(t, f)
}

func populateCopiedMasterRenditions(t *testing.T, f copiedHTTPFixture, qualities []string) {
	t.Helper()
	for _, quality := range qualities[1:] {
		for _, name := range []string{"init.mp4", "index.m3u8"} {
			data, err := os.ReadFile(filepath.Join(f.directory, "360p", name))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.directory, quality, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := writeCopiedCacheFile(f.config.CacheDir, path, data); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func masterRenditions(qualities []string) string {
	var master strings.Builder
	for _, quality := range qualities {
		master.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=1000000\n" + quality + "/index.m3u8\n")
	}
	return master.String()
}

func copiedMasterMetadata(t *testing.T, f copiedHTTPFixture) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.directory, "index.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			metadata.WriteString(line + "\n")
		}
	}
	return metadata.String()
}
