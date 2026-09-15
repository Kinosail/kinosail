package downloads

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsUnsafePersistedState(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	root := filepath.Join(cache, "downloads")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	unsafe := `{"id":"aaaaaaaaaaaaaaaa","itemId":"item","profileId":"viewer","title":"Film","quality":"original","state":"preparing","extension":"../../outside","created":"2026-08-29T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(root, "aaaaaaaaaaaaaaaa.json"), []byte(unsafe), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Config{Context: t.Context(), Cache: cache, Persist: persistJSON})
	if manager.Err() == nil || len(manager.jobs) != 0 {
		t.Fatalf("unsafe state was accepted: err=%v jobs=%d", manager.Err(), len(manager.jobs))
	}
}

func TestNewRecoversInterruptedAndCorruptJobs(t *testing.T) { //nolint:cyclop,gocognit // The table covers each persisted recovery state.
	t.Parallel()
	for _, test := range []struct {
		name, state, digest string
		size                int64
	}{
		{"interrupted", "preparing", "", 0},
		{"corrupt", "ready", strings.Repeat("a", 64), 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cache := t.TempDir()
			root := filepath.Join(cache, "downloads")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			job := Job{ID: "aaaaaaaaaaaaaaaa", ItemID: "item", Profile: "viewer", Title: "Film", Quality: "original", State: test.state, SHA256: test.digest, Size: test.size, ReadyOffline: test.state == "ready", Extension: ".mp4", Created: time.Now()}
			data, err := json.Marshal(job)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, job.ID+".json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, job.ID+job.Extension), []byte("wrong"), 0o600); err != nil {
				t.Fatal(err)
			}
			manager := New(Config{Context: t.Context(), Cache: cache, Persist: persistJSON})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			_, _ = manager.Wait(ctx, "viewer", job.ID)
			recovered, found := manager.Get("viewer", job.ID)
			if manager.Err() != nil || !found || recovered.State != "failed" || recovered.Error == "" || recovered.ReadyOffline {
				t.Fatalf("recovered = %#v, found=%v, err=%v", recovered, found, manager.Err())
			}
		})
	}
}

func TestNewCleansStagedDeletionAndIgnoresOtherFiles(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	root := filepath.Join(cache, "downloads")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "old.deleting")
	if err := os.WriteFile(staged, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Config{Context: t.Context(), Cache: cache, Persist: persistJSON})
	if manager.Err() != nil || len(manager.jobs) != 0 {
		t.Fatalf("load = %v, jobs=%d", manager.Err(), len(manager.jobs))
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged deletion remains: %v", err)
	}
}

func TestJobValidationRejectsMalformedFields(t *testing.T) {
	t.Parallel()
	valid := Job{ID: "aaaaaaaaaaaaaaaa", ItemID: "item", Profile: "viewer", Title: "Film", Quality: "original", State: "preparing", Extension: ".mp4", Created: time.Now()}
	if !validJob(valid) {
		t.Fatal("valid job was rejected")
	}
	mutations := []func(*Job){
		func(job *Job) { job.ID = "bad" },
		func(job *Job) { job.ItemID = "" },
		func(job *Job) { job.Profile = strings.Repeat("p", 257) },
		func(job *Job) { job.Title = "" },
		func(job *Job) { job.Quality = "bad" },
		func(job *Job) { job.State = "bad" },
		func(job *Job) { job.Extension = "../bad" },
		func(job *Job) { job.Created = time.Time{} },
		func(job *Job) { job.Size = -1 },
		func(job *Job) { job.Error = "unexpected" },
	}
	for position, mutate := range mutations {
		job := valid
		mutate(&job)
		if validJob(job) {
			t.Errorf("mutation %d was accepted: %#v", position, job)
		}
	}
}
