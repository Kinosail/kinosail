package downloads

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestLifetimeCapacityRejectsBeforePersistence(t *testing.T) {
	manager := startManager(t, t.Context())
	for index := range maximumJobs {
		job := validStateJob()
		job.ID = fmt.Sprintf("%016x", index)
		manager.jobs[job.ID] = job
	}
	var writes atomic.Int32
	manager.persist = func(string, any) error { writes.Add(1); return nil }
	if _, err := manager.Start("viewer", downloadItem(), "original"); !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity error = %v", err)
	}
	if writes.Load() != 0 || len(manager.pending) != 0 || len(manager.jobs) != maximumJobs {
		t.Fatal("rejected admission changed state")
	}
	files, err := os.ReadDir(manager.root)
	if err != nil || len(files) != 0 {
		t.Fatalf("rejected admission wrote files: %v, %v", files, err)
	}
}

func TestRemoveCancelsPreparationAndSameIdentityCanRestart(t *testing.T) { //nolint:cyclop // Cancellation and restart must exercise the same download identity.
	input := filepath.Join(t.TempDir(), "movie.mp4")
	if err := os.WriteFile(input, []byte("original media"), 0o600); err != nil {
		t.Fatal(err)
	}
	entered, canceled := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Persist: persistJSON, Acquire: func(ctx context.Context) (func(), error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-ctx.Done()
			close(canceled)
			return nil, ctx.Err()
		}
		return func() {}, nil
	}})
	item := library.Item{ID: "movie", Kind: "video", Title: "Movie", Path: input, Added: time.Now()}
	job, err := manager.Start("viewer", item, "original")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("preparation did not start")
	}
	if err := manager.Remove("viewer", job.ID); err != nil {
		t.Fatal(err)
	}
	next, err := manager.Start("viewer", item, "original")
	if err != nil || next.ID != job.ID {
		t.Fatalf("restart = %#v, %v", next, err)
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("removal did not cancel preparation")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ready, err := manager.Wait(ctx, "viewer", next.ID)
	if err != nil || !ready.ReadyOffline {
		t.Fatalf("replacement = %#v, %v", ready, err)
	}
	data, err := os.ReadFile(ready.File)
	if err != nil || string(data) != "original media" {
		t.Fatalf("replacement bytes = %q, %v", data, err)
	}
}

func TestCopyAndSealUseSameIntegrityContract(t *testing.T) { //nolint:cyclop // Copy, verification, and rejection assertions share one source fixture.
	root := t.TempDir()
	input, output := filepath.Join(root, "input"), filepath.Join(root, "output")
	if err := os.WriteFile(input, []byte(strings.Repeat("media", int(ChunkSize)/5+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	copied, err := copyManifest(t.Context(), input, output, "aaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealManifestContext(t.Context(), output, copied.ID)
	if err != nil || sealed.SHA256 != copied.SHA256 || sealed.Size != copied.Size || len(copied.Chunks) != 2 {
		t.Fatalf("manifest = %#v, %#v, %v", copied, sealed, err)
	}
	for index := range copied.Chunks {
		if copied.Chunks[index] != sealed.Chunks[index] {
			t.Fatal("block digest mismatch")
		}
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	rejected := filepath.Join(root, "rejected")
	if _, err := copyManifest(canceled, input, rejected, copied.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	if _, err := os.Stat(rejected); !os.IsNotExist(err) {
		t.Fatalf("canceled copy wrote output: %v", err)
	}
	if _, err := copyManifest(t.Context(), input, rejected, "../escape"); err == nil {
		t.Fatal("invalid identity accepted")
	}
	if _, err := os.Stat(rejected); !os.IsNotExist(err) {
		t.Fatal("invalid identity wrote output")
	}
}

func TestRestoredMediaIsUnavailableUntilVerified(t *testing.T) { //nolint:cyclop,gocognit // Each restored-state case checks preparation, verification, and final visibility.
	for _, corrupt := range []bool{false, true} {
		t.Run(strconv.FormatBool(corrupt), func(t *testing.T) {
			manager := startManager(t, t.Context())
			job := validStateJob()
			job.State, job.Error, job.ReadyOffline = "ready", "", true
			job.File = filepath.Join(manager.root, job.ID+job.extension())
			if err := os.WriteFile(job.File, []byte("sealed media"), 0o600); err != nil {
				t.Fatal(err)
			}
			manifest, err := sealManifestContext(t.Context(), job.File, job.ID)
			if err != nil {
				t.Fatal(err)
			}
			job.Size, job.SHA256 = manifest.Size, manifest.SHA256
			if err := manager.save(job); err != nil {
				t.Fatal(err)
			}
			if corrupt {
				if err := os.WriteFile(job.File, []byte("broken media"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := manager.load(); err != nil {
				t.Fatal(err)
			}
			restored, found := manager.Get("viewer", job.ID)
			if !found || restored.ReadyOffline || restored.State != "preparing" {
				t.Fatalf("unverified delivery = %#v", restored)
			}
			manager.verifyRestored()
			restored, _ = manager.Get("viewer", job.ID)
			if restored.ReadyOffline == corrupt || (corrupt && restored.State != "failed") {
				t.Fatalf("verified result = %#v", restored)
			}
		})
	}
}
