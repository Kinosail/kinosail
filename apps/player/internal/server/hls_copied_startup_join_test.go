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

// The real-codec public proof exposed a missing fragment at startup cancellation.
// Isolate the otherwise unschedulable join boundary: canceled workers must not be
// adopted, replacement must wait for join, rejected requests must have no effects,
// and a live producer must remain adoptable without replacing retained output.
func TestCopiedStartupCanceledProducerJoinsBeforeHTTPRefill(t *testing.T) {
	manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
	check := copiedStartupRetained(t, directory, item.Path)
	marker := filepath.Join(t.TempDir(), "refills")
	copiedRecoveryEncoderOutput(t, manager, "printf refill\\n >> "+copiedRecoveryQuote(marker), initialization, "first fragment", copiedRecoveryManifest)
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
	joined := false
	join := func() {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		if !joined {
			delete(manager.jobs, key)
			close(job.done)
			joined = true
		}
	}
	t.Cleanup(join)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	response := httptest.NewRecorder()
	done := make(chan struct{})
	entered := make(chan struct{})
	go func() {
		close(entered)
		manager.serveRecipe(response, httptest.NewRequestWithContext(ctx, http.MethodGet, "/hls/"+item.ID+"/p/"+recipe.token()+"/1080p/segment-00000.m4s", nil), item, recipe, "1080p/segment-00000.m4s")
		close(done)
	}()
	<-entered
	select {
	case <-done:
		t.Fatal("missing-fragment request completed before canceled producer joined")
	case <-time.After(100 * time.Millisecond):
	}
	if job.preparation.adopted.Load() {
		t.Fatal("canceled copied producer was adopted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("replacement started before old producer joined")
	}
	join()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("joined producer did not refill the public fragment")
	}
	if response.Code != http.StatusOK || response.Body.String() != "first fragment" {
		t.Fatalf("refill = %d %q", response.Code, response.Body.String())
	}
	manager.mu.Lock()
	replacement := manager.jobs[key]
	manager.mu.Unlock()
	if replacement != nil {
		if err := waitForReplacedHLSJob(ctx, replacement); err != nil {
			t.Fatal(err)
		}
	}
	used, err := os.ReadFile(marker)
	if err != nil || strings.Count(string(used), "refill") != 1 {
		t.Fatalf("refill count = %q, %v", used, err)
	}
	if job.preparation.adopted.Load() {
		t.Fatal("joined canceled producer was adopted late")
	}
	check()
}

func TestCopiedStartupWaitingRequestCancellationHasNoEffects(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "deadline"}[deadline], func(t *testing.T) {
			manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
			check := copiedStartupRetained(t, directory, item.Path)
			marker := filepath.Join(t.TempDir(), "refills")
			copiedRecoveryEncoderOutput(t, manager, "printf started > "+copiedRecoveryQuote(marker), initialization, "first fragment", copiedRecoveryManifest)
			policy, err := manager.hlsSettings(item, recipe)
			if err != nil {
				t.Fatal(err)
			}
			lifecycle, stop := context.WithCancelCause(t.Context())
			stop(errHLSInactive)
			job := &hlsJob{lifecycle: lifecycle, cancel: stop, preparation: &startupEncoding{}, done: make(chan struct{}), cachePolicy: policy.Cache}
			key := hlsRecipeKey(item.ID, recipe)
			manager.jobs[key] = job
			ctx, cancel := context.WithCancel(t.Context())
			if deadline {
				ctx, cancel = context.WithTimeout(t.Context(), 100*time.Millisecond)
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- manager.prepareSegment(ctx, item, recipe, "1080p/segment-00000.m4s") }()
			if !deadline {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}
			select {
			case err := <-done:
				if err != ctx.Err() || err == nil {
					t.Fatalf("canceled request = %v, context = %v", err, ctx.Err())
				}
			case <-time.After(time.Second):
				t.Fatal("canceled request failed to settle without producer join")
			}
			if job.preparation.adopted.Load() {
				t.Fatal("rejected request adopted canceled producer")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("rejected request started a replacement")
			}
			check()
		})
	}
}

func TestCopiedStartupLiveProducerRemainsAdoptable(t *testing.T) {
	manager, item, recipe, directory, _ := copiedRecoveryEnclosingFixture(t)
	check := copiedStartupRetained(t, directory, item.Path)
	policy, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, stop := context.WithCancelCause(t.Context())
	defer stop(nil)
	job := &hlsJob{lifecycle: lifecycle, cancel: stop, preparation: &startupEncoding{}, done: make(chan struct{}), cachePolicy: policy.Cache}
	manager.jobs[hlsRecipeKey(item.ID, recipe)] = job
	if err := manager.prepareSegment(t.Context(), item, recipe, "1080p/segment-00000.m4s"); err != nil {
		t.Fatal(err)
	}
	if !job.preparation.adopted.Load() {
		t.Fatal("live copied producer was not adopted")
	}
	if _, err := os.Stat(filepath.Join(directory, "1080p/segment-00000.m4s")); !os.IsNotExist(err) {
		t.Fatal("adopting live output invented a fragment")
	}
	check()
}

func TestCopiedStartupChangedPolicyRejectsBeforeAdoption(t *testing.T) {
	manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
	writeHLSLoadingFile(t, filepath.Join(directory, ".source"), "untrusted changed policy")
	check := copiedStartupRetained(t, directory, item.Path)
	marker := filepath.Join(t.TempDir(), "refills")
	copiedRecoveryEncoderOutput(t, manager, "printf started > "+copiedRecoveryQuote(marker), initialization, "first fragment", copiedRecoveryManifest)
	lifecycle, stop := context.WithCancelCause(t.Context())
	defer stop(nil)
	job := &hlsJob{lifecycle: lifecycle, cancel: stop, preparation: &startupEncoding{}, done: make(chan struct{})}
	key := hlsRecipeKey(item.ID, recipe)
	manager.jobs[key] = job
	if err := manager.prepareSegment(t.Context(), item, recipe, "1080p/segment-00000.m4s"); err == nil {
		t.Fatal("changed policy admitted refill")
	}
	if job.preparation.adopted.Load() || manager.jobs[key] != job {
		t.Fatal("rejected policy changed the live producer")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("rejected policy started an encoder")
	}
	check()
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
