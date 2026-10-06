package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// A retained source identity is valid, but media from before the AAC refill
// repair cannot acquire the corrected packet/clock contract through reuse.
func TestCopiedHLSHTTPInvalidatesPriorRefillPolicy(t *testing.T) {
	for _, prior := range []string{"hls=16", "hls=15"} {
		t.Run(prior, func(t *testing.T) {
			for _, mode := range []string{"playlist journey", "direct cached segment", "direct cached init", "changed source segment", "changed source init"} {
				t.Run(mode, func(t *testing.T) { assertPriorCopiedPolicyRejected(t, mode, prior) })
			}
		})
	}
}

// A valid init request must not rebuild the cold startup window when an
// independently refillable fragment and optional seek marker were evicted.
// The real codec owner verifies
// payload continuity; this process fixture makes the no-new-encoder boundary
// deterministic even when the old startup worker has already become inactive.
func TestCopiedHLSHTTPCachedInitializationSurvivesFragmentEviction(t *testing.T) {
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	path := strings.TrimSuffix(f.source, "index.m3u8") + "360p/init.mp4"
	if err := os.Remove(filepath.Join(f.directory, "360p", "segment-00000.m4s")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.directory, ".seekable")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	before := snapshotCopiedPolicyCache(t, f, false)
	initialization, err := os.ReadFile(filepath.Join(f.directory, "360p", "init.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := server.New(f.config)
	response := apiCall(t, restarted, "", http.MethodGet, path, nil)
	assertAPIBody(t, response, http.StatusOK)
	if !bytes.Equal(initialization, response.Body.Bytes()) || !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, false)) {
		t.Fatal("valid cached initialization request rebuilt media or admitted encoding")
	}
}

func TestCopiedHLSHTTPColdInvalidAssetsDoNotAdmitEncoding(t *testing.T) {
	for _, name := range []string{"segment-99999.m4s", "segment--1.m4s", "unknown.m4s"} {
		t.Run(name, func(t *testing.T) {
			f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			response := httptest.NewRecorder()
			path := strings.TrimSuffix(f.source, "index.m3u8") + "360p/" + name
			f.handler.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))
			assertAPIBody(t, response, http.StatusNotFound)
			if _, err := os.Stat(f.starts); !os.IsNotExist(err) {
				t.Fatalf("invalid cold asset started encoding: %v", err)
			}
			if _, err := os.Stat(f.directory); !os.IsNotExist(err) {
				t.Fatalf("invalid cold asset created media/cache: %v", err)
			}
		})
	}
}

func TestCopiedHLSHTTPCachedInvalidAssetsRejectWithoutMutation(t *testing.T) {
	for _, value := range []struct{ name, certificate string }{
		{"segment-99999.m4s", ""},
		{"segment-00002.m4s", "not-json"},
		{"segment-00002.m4s", strings.Repeat("x", (256<<10)+1)},
	} {
		t.Run(value.name+value.certificate[:min(8, len(value.certificate))], func(t *testing.T) {
			f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
			awaitCopiedReady(t, f)
			if value.certificate != "" {
				if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, ".copy-timeline"), []byte(value.certificate)); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, "360p", value.name), []byte("uncertified cached media")); err != nil {
				t.Fatal(err)
			}
			before := snapshotCopiedPolicyCache(t, f, true)
			path := strings.TrimSuffix(f.source, "index.m3u8") + "360p/" + value.name
			response := apiCall(t, f.handler, "", http.MethodGet, path, nil)
			assertAPIBody(t, response, http.StatusNotFound)
			if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, true)) {
				t.Fatal("rejected cached asset mutated media or admitted encoding")
			}
		})
	}
}

func snapshotCopiedPolicyCache(t *testing.T, f copiedHTTPFixture, preparationMarker bool) []byte {
	t.Helper()
	root, err := os.OpenRoot(f.directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	var data []byte
	if err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Successful media adoption removes .startup; rejected requests must not.
		if !preparationMarker && path == ".startup" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data = fmt.Appendf(data, "%q %s\n", path, info.Mode())
		if entry.IsDir() {
			return nil
		}
		data = fmt.Appendf(data, "%d\n", info.Size())
		value, err := root.ReadFile(path)
		if err != nil {
			return err
		}
		data = fmt.Appendf(data, "%x\n", sha256.Sum256(value))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	starts, err := os.ReadFile(f.starts)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, starts...)
}

func assertPriorCopiedPolicyRejected(t *testing.T, mode, prior string) {
	t.Helper()
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	name, expected := preparePriorCopiedCache(t, f, mode, prior)
	before, err := os.ReadFile(f.starts)
	if err != nil {
		t.Fatal(err)
	}
	restarted := server.New(f.config)
	base := strings.TrimSuffix(f.source, "index.m3u8") + "360p/"
	if mode == "playlist journey" {
		assertAPIBody(t, apiCall(t, restarted, "", http.MethodGet, f.source, nil), http.StatusOK, "360p/index.m3u8")
		assertAPIBody(t, apiCall(t, restarted, "", http.MethodGet, base+"index.m3u8", nil), http.StatusOK, "segment-00002.m4s")
	}
	response := apiCall(t, restarted, "", http.MethodGet, base+name, nil)
	if strings.HasSuffix(mode, "segment") {
		assertAPIBody(t, response, http.StatusNotFound)
		unchanged, err := os.ReadFile(f.starts)
		if err != nil || !bytes.Equal(before, unchanged) {
			t.Fatal("stale direct segment allocated encoding")
		}
		assertAPIBody(t, apiCall(t, restarted, "", http.MethodGet, f.source, nil), http.StatusOK, "360p/index.m3u8")
		response = apiCall(t, restarted, "", http.MethodGet, base+name, nil)
	}
	assertAPIBody(t, response, http.StatusOK, string(expected))
	if strings.Contains(response.Body.String(), "old refill policy media") {
		t.Fatal("retained policy reused uncorrected AAC refill media")
	}
	after, err := os.ReadFile(f.starts)
	if err != nil || bytes.Equal(before, after) {
		t.Fatal("prior refill policy was not re-encoded")
	}
}

func preparePriorCopiedCache(t *testing.T, f copiedHTTPFixture, mode, prior string) (string, []byte) {
	t.Helper()
	if strings.HasPrefix(mode, "changed source") {
		if err := os.WriteFile(filepath.Join(f.config.MediaDir, "Copied.mp4"), []byte("changed owned source"), 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		binding, err := os.ReadFile(filepath.Join(f.directory, ".source"))
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Split(strings.TrimSpace(string(binding)), ":")
		current := fields[len(fields)-1]
		for _, name := range []string{".source", ".copy-timeline", "index.m3u8"} {
			path := filepath.Join(f.directory, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.ReplaceAll(data, []byte(current), []byte(prior))
			if err := writeCopiedCacheFile(f.config.CacheDir, path, data); err != nil {
				t.Fatal(err)
			}
		}
	}
	name := "segment-00002.m4s"
	expected := []byte("fragment-2")
	if strings.HasSuffix(mode, "init") {
		name = "init.mp4"
		var err error
		expected, err = os.ReadFile(filepath.Join(f.directory, "360p", name))
		if err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(f.directory, "360p", name)
	if err := writeCopiedCacheFile(f.config.CacheDir, path, []byte("old refill policy media")); err != nil {
		t.Fatal(err)
	}
	return name, expected
}
