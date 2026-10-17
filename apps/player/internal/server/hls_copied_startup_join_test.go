package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	join := copiedStartupJoin(t, manager, key, job)
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
	copiedStartupNoRefill(t, job, marker)
	join()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("joined producer did not refill the public fragment")
	}
	if response.Code != http.StatusOK || response.Body.String() != "first fragment" {
		t.Fatalf("refill = %d %q", response.Code, response.Body.String())
	}
	copiedStartupRefillSettled(t, ctx, manager, key, marker)
	if job.preparation.adopted.Load() {
		t.Fatal("joined canceled producer was adopted late")
	}
	check()
}

func TestCopiedStartupWaitingRequestCancellationHasNoEffects(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "deadline"}[deadline], func(t *testing.T) {
			copiedStartupCancellation(t, deadline)
		})
	}
}

func copiedStartupCancellation(t *testing.T, deadline bool) {
	t.Helper()
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
		if err == nil || !errors.Is(err, ctx.Err()) {
			t.Fatalf("canceled request = %v, context = %v", err, ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request failed to settle without producer join")
	}
	copiedStartupNoRefill(t, job, marker)
	check()
}

func copiedStartupJoin(t *testing.T, manager *hlsManager, key string, job *hlsJob) func() {
	t.Helper()
	joined := false
	join := func() {
		job.cancel(errHLSEncodeComplete)
		manager.mu.Lock()
		defer manager.mu.Unlock()
		if !joined {
			delete(manager.jobs, key)
			close(job.done)
			joined = true
		}
	}
	t.Cleanup(join)
	return join
}

func copiedStartupNoRefill(t *testing.T, job *hlsJob, marker string) {
	t.Helper()
	if job.preparation.adopted.Load() {
		t.Fatal("request adopted canceled producer before join")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("request started a replacement before join")
	}
}

func copiedStartupRefillSettled(t *testing.T, ctx context.Context, manager *hlsManager, key, marker string) {
	t.Helper()
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
}

func TestCopiedStartupLiveProducerRemainsAdoptable(t *testing.T) {
	for _, outcome := range []string{"published", "completed", "canceled", "source-changed", "source-changed-published"} {
		t.Run(outcome, func(t *testing.T) { copiedStartupLiveHTTP(t, outcome) })
	}
}

func copiedStartupLiveHTTP(t *testing.T, outcome string) {
	t.Helper()
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
	job := &hlsJob{lifecycle: lifecycle, cancel: stop, preparation: &startupEncoding{}, done: make(chan struct{}), cachePolicy: policy.Cache}
	key := hlsRecipeKey(item.ID, recipe)
	manager.jobs[key] = job
	join := copiedStartupJoin(t, manager, key, job)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		manager.serveRecipe(response, httptest.NewRequestWithContext(ctx, http.MethodGet, "/hls/"+item.ID+"/p/"+recipe.token()+"/1080p/segment-00000.m4s", nil), item, recipe, "1080p/segment-00000.m4s")
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		copiedStartupAwaitRequest(t, done, "cancellation")
	})
	fragment := filepath.Join(directory, "1080p/segment-00000.m4s")
	copiedStartupAwaitAdoption(t, job, fragment, marker, done)
	check()
	status, check := copiedStartupPublishOutcome(t, outcome, directory, item.Path, join, cancel)
	copiedStartupAwaitRequest(t, done, "publication, join or cancellation")
	if response.Code != status || status == http.StatusOK && response.Body.String() != "first fragment" {
		t.Fatalf("live fragment = %d; expected %d with retained bytes", response.Code, status)
	}
	if outcome == "completed" {
		copiedStartupRefillSettled(t, ctx, manager, key, marker)
	} else if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("live publication or rejected request started a replacement")
	}
	check()
}

func copiedStartupPublishOutcome(t *testing.T, outcome, directory, source string, join, cancel func()) (int, func()) {
	t.Helper()
	fragment := filepath.Join(directory, "1080p/segment-00000.m4s")
	check := copiedStartupRetained(t, directory, source)
	status := http.StatusOK
	switch outcome {
	case "published":
		publishHLSLoadingFile(t, fragment, "first fragment")
	case "completed":
		join()
	case "canceled":
		status = http.StatusNotFound
		cancel()
	case "source-changed", "source-changed-published":
		status = http.StatusNotFound
		writeHLSLoadingFile(t, source, "changed source")
		check = copiedStartupRetained(t, directory, source)
		if outcome == "source-changed-published" {
			publishHLSLoadingFile(t, fragment, "first fragment")
		} else {
			join()
		}
	}
	return status, check
}

func copiedStartupAwaitRequest(t *testing.T, done <-chan struct{}, action string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("live producer request failed to settle after %s", action)
	}
}

func copiedStartupAwaitAdoption(t *testing.T, job *hlsJob, fragment, marker string, done <-chan struct{}) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for !job.preparation.adopted.Load() && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if !job.preparation.adopted.Load() {
		t.Fatal("live copied producer was not adopted")
	}
	if _, err := os.Stat(fragment); !os.IsNotExist(err) {
		t.Fatal("adopting live output invented a fragment")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("adopting live output started a replacement")
	}
	select {
	case <-done:
		t.Fatal("live missing fragment completed before publication or join")
	default:
	}
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

func publishHLSLoadingFile(t *testing.T, path, data string) {
	t.Helper()
	writeHLSLoadingFile(t, path+".tmp", data)
	if err := os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
}
