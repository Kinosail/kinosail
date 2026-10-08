package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// These isolated controls cover source/route, cancellation and replacement paths
// absent from the public ten-second fixture; fake assets do not prove AAC content.
func TestRemainingColdAACManifestCannotSubstituteJoinForEOF(t *testing.T) {
	for _, state := range []string{"complete", "incomplete-event", "short-false-eof", "missing-tail", "copy-index"} {
		t.Run(state, func(t *testing.T) {
			checkRemainingColdAACManifestState(t, state)
		})
	}
}

func checkRemainingColdAACManifestState(t *testing.T, state string) {
	t.Helper()
	manager, item, recipe, directory := remainingAACAdmissionFixture(t)
	key := hlsRecipeKey(item.ID, recipe)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	job := &hlsJob{done: make(chan struct{}), cachePolicy: options.Cache}
	close(job.done)
	manager.jobs[key] = job
	manifest := remainingInitialAACPrefix + "#EXTINF:0.021333,\nsegment-00005.m4s\n#EXT-X-ENDLIST\n"
	seedRemainingColdAACSegments(t, directory)
	manifest = faultRemainingColdAACManifest(t, state, directory, manifest)
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), manifest)
	snapshot, err := manager.remainingColdAACSnapshot(t.Context(), item, recipe, key, options.Cache, job, 10)
	if state == "complete" {
		if err != nil || !bytes.Equal(snapshot.manifest, []byte(manifest)) {
			t.Fatalf("complete EOF rejected: %v", err)
		}
	} else if err == nil {
		t.Fatal("incomplete or unindexed output claimed complete EOF")
	}
}

func TestRemainingColdAACProjectionRejectsPublicationReplacement(t *testing.T) {
	for _, changed := range []string{"none", "source", "policy", "root", "rendition", "manifest", "init", "tail", "replacement-job"} {
		t.Run(changed, func(t *testing.T) {
			checkRemainingColdAACPublicationState(t, changed)
		})
	}
}

func checkRemainingColdAACPublicationState(t *testing.T, changed string) {
	t.Helper()
	manager, item, recipe, directory := remainingAACAdmissionFixture(t)
	key := hlsRecipeKey(item.ID, recipe)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	manifest := remainingInitialAACPrefix + "#EXTINF:0.021333,\nsegment-00005.m4s\n#EXT-X-ENDLIST\n"
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), manifest)
	seedRemainingColdAACSegments(t, directory)
	job := &hlsJob{done: make(chan struct{}), cachePolicy: options.Cache}
	close(job.done)
	manager.jobs[key] = job
	projection, err := manager.remainingColdAACProjection(t.Context(), item, recipe, key, "audio/index.m3u8", http.MethodGet, 10)
	if err != nil || projection == nil {
		t.Fatalf("complete publication rejected: %v", err)
	}
	replaceRemainingColdAACPublication(t, changed, manager, item, key, directory, options.Cache)
	delivered := projection([]byte(manifest))
	if changed == "none" {
		if delivered == nil || !bytes.Contains(delivered, []byte("segment-00005.m4s")) {
			t.Fatal("final observed tail disappeared")
		}
	} else if delivered != nil {
		t.Fatal("replaced publication inherited prior admission")
	}
}

func TestRemainingColdAACConcurrentReadersShareJoinedPublication(t *testing.T) {
	manager, item, recipe, directory := remainingAACAdmissionFixture(t)
	key := hlsRecipeKey(item.ID, recipe)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	manifest := remainingInitialAACPrefix + "#EXTINF:0.021333,\nsegment-00005.m4s\n#EXT-X-ENDLIST\n"
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), manifest)
	seedRemainingColdAACSegments(t, directory)
	_ = manager.probe.facts(t.Context(), item)
	job := &hlsJob{done: make(chan struct{}), cachePolicy: options.Cache, activity: make(chan struct{}, 2)}
	manager.jobs[key] = job
	completed := make(chan error, 2)
	var readers sync.WaitGroup
	for range 2 {
		readers.Go(func() {
			completed <- remainingColdAACReaderProjection(t.Context(), manager, item, recipe, key, manifest)
		})
	}
	checkRemainingColdAACPendingReaders(t, manager, key, job, completed)
	close(job.done)
	readers.Wait()
	for range 2 {
		if err := <-completed; err != nil {
			t.Fatal(err)
		}
	}
	if manager.jobs[key] != job || len(job.activity) != 0 || job.preparation != nil {
		t.Fatal("reader changed shared encoder ownership")
	}
}

func replaceRemainingColdAACPublication(t *testing.T, changed string, manager *hlsManager, item library.Item, key, directory, policy string) {
	t.Helper()
	switch changed {
	case "source":
		writeHLSLoadingFile(t, item.Path, "different-source")
	case "policy":
		writeHLSLoadingFile(t, filepath.Join(directory, ".source"), "different-policy")
	case "replacement-job":
		manager.jobs[key] = &hlsJob{done: make(chan struct{}), cachePolicy: policy}
	case "root", "rendition":
		target := directory
		if changed == "rendition" {
			target = filepath.Join(directory, "audio")
		}
		if err := os.Rename(target, target+".previous"); err != nil {
			t.Fatal(err)
		}
		cloneRemainingColdFixture(t, target+".previous", target)
	case "manifest", "init", "tail":
		name := map[string]string{"manifest": "index.m3u8", "init": "init.mp4", "tail": "segment-00005.m4s"}[changed]
		target := filepath.Join(directory, "audio", name)
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(target, target+".previous"); err != nil {
			t.Fatal(err)
		}
		writeHLSLoadingFile(t, target, string(data))
	}
}

func remainingColdAACReaderProjection(ctx context.Context, manager *hlsManager, item library.Item, recipe hlsRecipe, key, manifest string) error {
	projection, problem := manager.remainingColdAACProjection(ctx, item, recipe, key, "audio/index.m3u8", http.MethodGet, 10)
	if problem == nil && (projection == nil || !bytes.Contains(projection([]byte(manifest)), []byte("segment-00005.m4s"))) {
		return errors.New("concurrent reader lost final cut")
	}
	return problem
}

func faultRemainingColdAACManifest(t *testing.T, state, directory, manifest string) string {
	t.Helper()
	switch state {
	case "incomplete-event":
		manifest = strings.TrimSuffix(manifest, "#EXT-X-ENDLIST\n")
	case "short-false-eof":
		manifest = strings.Split(remainingInitialAACPrefix, "#EXTINF:2.005333,\nsegment-00004")[0] + "#EXT-X-ENDLIST\n"
	case "missing-tail":
		if err := os.Remove(filepath.Join(directory, "audio/segment-00005.m4s")); err != nil {
			t.Fatal(err)
		}
	case "copy-index":
		writeHLSLoadingFile(t, filepath.Join(directory, ".copy-timeline"), "{}")
	}
	return manifest
}

func checkRemainingColdAACPendingReaders(t *testing.T, manager *hlsManager, key string, job *hlsJob, completed <-chan error) {
	t.Helper()
	waitRemainingColdAACReaders(t, 2)
	// A pending worker must leave the metadata lock available to other requests.
	select {
	case <-job.done:
		t.Fatal("worker joined before the lock oracle")
	default:
	}
	unlocked := make(chan bool, 1)
	go func() {
		manager.mu.Lock()
		same := manager.jobs[key] == job
		manager.mu.Unlock()
		unlocked <- same
	}()
	select {
	case same := <-unlocked:
		if !same {
			t.Fatal("pending reader changed the shared worker")
		}
	case <-time.After(time.Second):
		t.Fatal("completion wait retained the manager mutex")
	}
	select {
	case err := <-completed:
		t.Fatalf("reader escaped before worker join: %v", err)
	default:
	}
}

func seedRemainingColdAACSegments(t *testing.T, directory string) {
	t.Helper()
	for n := range 6 {
		writeHLSLoadingFile(t, filepath.Join(directory, "audio", fmtRemainingColdSegment(n)), "synthetic-fragment")
	}
}
