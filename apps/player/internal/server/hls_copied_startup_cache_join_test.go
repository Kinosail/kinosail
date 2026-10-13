package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The public real-codec proof exposed this unschedulable publication boundary.
// A canceled startup producer must finish publishing its preservation marker
// before playback decides whether to reuse or replace its certified cache.
func TestCopiedStartupHTTPMasterRechecksCacheAfterProducerJoin(t *testing.T) {
	manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
	copiedStartupUnfinishedCache(t, directory)
	check := copiedStartupRetained(t, directory, item.Path)
	marker := filepath.Join(t.TempDir(), "replaced")
	copiedRecoveryEncoderOutput(t, manager, "printf started > "+copiedRecoveryQuote(marker), initialization, "first fragment", copiedRecoveryManifest)
	policy, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, stop := context.WithCancelCause(t.Context())
	defer stop(nil)
	stop(errHLSInactive)
	job := &hlsJob{lifecycle: lifecycle, cancel: stop, preparation: &startupEncoding{}, done: make(chan struct{}), activity: make(chan struct{}, 1), cachePolicy: policy.Cache}
	key := hlsRecipeKey(item.ID, recipe)
	manager.jobs[key] = job
	join := copiedStartupJoin(t, manager, key, job)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if manager.reusableCopiedHLS(ctx, item, directory, policy.Cache, recipe) {
		t.Fatal("unfinished startup cache was reusable before preservation")
	}
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		manager.serveRecipe(response, httptest.NewRequestWithContext(ctx, http.MethodGet, "/hls/"+item.ID+"/p/"+recipe.token()+"/index.m3u8", nil), item, recipe, "index.m3u8")
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("master request completed before canceled producer joined")
	case <-time.After(100 * time.Millisecond):
	}
	copiedStartupNoRefill(t, job, marker)
	if !preserveInactiveHLS(job, directory, item.Path, policy.Cache) {
		t.Fatal("startup cancellation failed to publish its preservation marker")
	}
	join()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("joined producer did not serve the retained public master")
	}
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "1080p/index.m3u8") {
		t.Fatalf("master = %d %q", response.Code, response.Body.String())
	}
	copiedStartupNoRefill(t, job, marker)
	check()
}

func copiedStartupUnfinishedCache(t *testing.T, directory string) {
	t.Helper()
	writeHLSLoadingFile(t, filepath.Join(directory, "1080p/segment-00000.m4s"), "first fragment")
	writeHLSLoadingFile(t, filepath.Join(directory, "1080p/index.m3u8"), strings.ReplaceAll(copiedRecoveryManifest, "#EXT-X-ENDLIST\n", ""))
	if err := os.Remove(filepath.Join(directory, ".seekable")); err != nil {
		t.Fatal(err)
	}
}

func copiedStartupRetained(t *testing.T, directory, source string) func() {
	t.Helper()
	names := []string{"index.m3u8", ".source", ".copy-timeline", ".copy-clock", "1080p/index.m3u8", "1080p/init.mp4", "1080p/segment-00001.m4s"}
	paths := []string{source}
	for _, name := range names {
		paths = append(paths, filepath.Join(directory, name))
	}
	data, identities := make([][]byte, len(paths)), make([]os.FileInfo, len(paths))
	for i, path := range paths {
		var err error
		data[i], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		identities[i], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	return func() {
		t.Helper()
		for i, path := range paths {
			after, err := os.ReadFile(path)
			identity, statErr := os.Stat(path)
			if err != nil || statErr != nil || !bytes.Equal(data[i], after) || !sameCopiedHLSFile(identities[i], identity) {
				t.Errorf("refill changed retained %s", path)
			}
		}
	}
}
