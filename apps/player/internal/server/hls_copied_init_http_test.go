package server_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// Public index admission must not advertise an unusable EXT-X-MAP after cache
// eviction or corruption. Direct map requests also need recovery without first
// fetching a playlist. Real codec delivery owns decode correctness; this process
// fixture isolates damaged persisted bytes and checks the recovery boundary.
func TestCopiedHLSHTTPRecoversMissingOrInvalidInitialization(t *testing.T) {
	for _, journey := range []string{"direct map", "index then map", "prepare then map"} {
		for _, damaged := range []string{"missing", "empty", "truncated", "oversized"} {
			t.Run(journey+"/"+damaged, func(t *testing.T) {
				assertCopiedInitializationRecovery(t, journey, damaged)
			})
		}
	}
}

func assertCopiedInitializationRecovery(t *testing.T, journey, damaged string) {
	t.Helper()
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	initialization := filepath.Join(f.directory, "360p", "init.mp4")
	valid, err := os.ReadFile(initialization)
	if err != nil {
		t.Fatal(err)
	}
	damageCopiedInitialization(t, f, initialization, valid, damaged)
	unrelated := filepath.Join(f.config.CacheDir, "unrelated-cache")
	if err := os.WriteFile(unrelated, []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	restarted := server.New(f.config)
	if journey == "prepare then map" {
		prepared := f
		prepared.handler = restarted
		awaitCopiedReady(t, prepared)
		recovered, err := os.ReadFile(initialization)
		if err != nil || !bytes.Equal(valid, recovered) {
			t.Fatal("public preparation reported ready with a damaged initialization")
		}
	}
	base := strings.TrimSuffix(f.source, "index.m3u8") + "360p/"
	if journey == "index then map" {
		response := apiCall(t, restarted, "", http.MethodGet, base+"index.m3u8", nil)
		assertAPIBody(t, response, http.StatusOK, `#EXT-X-MAP:URI="init.mp4"`)
	}
	response := apiCall(t, restarted, "", http.MethodGet, base+"init.mp4", nil)
	assertAPIBody(t, response, http.StatusOK)
	if !bytes.Equal(valid, response.Body.Bytes()) {
		t.Fatal("public map returned damaged initialization instead of recovered media")
	}
	if data, err := os.ReadFile(unrelated); err != nil || string(data) != "retained" {
		t.Fatal("map recovery mutated unrelated cache")
	}
	assertCopiedRecoverySourceUnchanged(t, f)
}

func assertCopiedRecoverySourceUnchanged(t *testing.T, f copiedHTTPFixture) {
	t.Helper()
	if data, err := os.ReadFile(filepath.Join(f.config.MediaDir, "Copied.mp4")); err != nil || string(data) != "owned source" {
		t.Fatal("map recovery mutated original media")
	}
}

func damageCopiedInitialization(t *testing.T, f copiedHTTPFixture, initialization string, valid []byte, damaged string) {
	t.Helper()
	var err error
	if damaged == "missing" {
		err = os.Remove(initialization)
	} else {
		data := valid[:0]
		if damaged == "truncated" {
			data = valid[:16]
		}
		if damaged == "oversized" {
			data = bytes.Repeat([]byte{'x'}, (2<<20)+1)
		}
		err = writeCopiedCacheFile(f.config.CacheDir, initialization, data)
	}
	if err != nil {
		t.Fatal(err)
	}
}
