package downloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readyRepairJob(t *testing.T) (*Manager, Job) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "download.mp4")
	if err := os.WriteFile(path, []byte("verified download"), 0o600); err != nil {
		t.Fatal(err)
	}
	sealed, err := sealManifestContext(t.Context(), path, "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	job := Job{ID: sealed.ID, Profile: "viewer", State: "ready", File: path, Size: sealed.Size, SHA256: sealed.SHA256, Created: time.Now()}
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Persist: persistJSON})
	manager.jobs[job.ID] = job
	return manager, job
}

func TestManifestRepairsMissingMetadataAndReusesIt(t *testing.T) {
	manager, job := readyRepairJob(t)
	for range 2 {
		got, err := manager.ManifestContext(t.Context(), job.Profile, job.ID)
		if err != nil || got.SHA256 != job.SHA256 || got.Size != job.Size {
			t.Fatalf("repair = %#v, %v", got, err)
		}
	}
	if _, err := readManifest(job); err != nil {
		t.Fatalf("repair was not persisted: %v", err)
	}
	for _, tc := range []struct{ profile, id string }{{"other", job.ID}, {job.Profile, "bad"}, {job.Profile, "1111111111111111"}} {
		if _, err := manager.ManifestContext(t.Context(), tc.profile, tc.id); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unauthorized repair = %v", err)
		}
	}
}

func TestManifestRepairRejectsChangedBytes(t *testing.T) {
	manager, job := readyRepairJob(t)
	if err := os.WriteFile(job.File, []byte("tampered download"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ManifestContext(t.Context(), job.Profile, job.ID); err == nil {
		t.Fatal("changed download was resealed")
	}
	if _, err := os.Stat(job.File + ".manifest"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected repair persisted metadata: %v", err)
	}
}

func TestManifestRepairCancellationReleasesWaiter(t *testing.T) {
	manager, job := readyRepairJob(t)
	manager.manifestSlots = make(chan struct{}, 2)
	manager.manifestSlots <- struct{}{}
	manager.manifestSlots <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := manager.ManifestContext(ctx, job.Profile, job.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled repair = %v", err)
	}
	manager.manifestMu.Lock()
	remaining := len(manager.manifestWork)
	manager.manifestMu.Unlock()
	if remaining != 0 {
		t.Fatal("canceled waiter retained repair")
	}
}

func TestManifestRepairBoundsQueueAndSharesRevision(t *testing.T) {
	manager, job := readyRepairJob(t)
	manager.manifestWork = make(map[string]*manifestFlight)
	for index := range QueueCapacity {
		manager.manifestWork[string(rune(index))] = &manifestFlight{}
	}
	if _, err := manager.manifestFlight(job); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("unbounded repair queue: %v", err)
	}
	existing := &manifestFlight{done: make(chan struct{})}
	manager.manifestWork[job.ID] = existing
	got, err := manager.manifestFlight(job)
	if err != nil || got != existing || existing.waiters != 1 {
		t.Fatalf("shared repair = %p, %v", got, err)
	}
}
