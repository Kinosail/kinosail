package server_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
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

// An encoder can finish a short title before the readiness poll's first tick.
// Successful finalized output must be certified and retained, rather than reset
// by treating ordinary process completion as a changed stream identity.
func TestCopiedHLSHTTPCompletedEncoderRetainsReadyInitialization(t *testing.T) {
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	if err := os.WriteFile(f.release, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	queued := prepareSource(t, f.handler, f.id, f.source)
	assertPreparationState(t, queued, http.StatusAccepted, "queued")
	awaitCopiedLog(t, f.output, "HLS startup preparation", queued.Header().Get("X-Request-ID"), "ready")
	assertPreparationState(t, prepareSource(t, f.handler, f.id, f.source), http.StatusAccepted, "ready")
	initialization := apiCall(t, f.handler, "", http.MethodGet, strings.TrimSuffix(f.source, "index.m3u8")+"360p/init.mp4", nil)
	assertAPIBody(t, initialization, http.StatusOK)
	if !bytes.Equal(initialization.Body.Bytes(), mp4fixture.Initialization(640, 360, "h264", "aac", "")) {
		t.Fatal("completed preparation lost its initialization")
	}
	encodes, err := os.ReadFile(f.starts)
	if err != nil || bytes.Count(encodes, []byte("encode\n")) != 1 {
		t.Fatal("completed preparation re-encoded valid output")
	}
	assertCopiedRecoverySourceUnchanged(t, f)
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
			t.Fatalf("public preparation reported ready with a damaged initialization: read_failed=%t bytes=%d expected_bytes=%d", err != nil, len(recovered), len(valid))
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
