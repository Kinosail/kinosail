package downloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestStartReportsStorageStartupAndContextFailures(t *testing.T) {
	item := downloadItem()
	t.Run("cache directory", func(t *testing.T) {
		blocked := filepath.Join(t.TempDir(), "blocked")
		if err := os.WriteFile(blocked, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		manager := startManager(t, context.Background())
		manager.root = filepath.Join(blocked, "downloads")
		if _, err := manager.Start("viewer", item, "original"); err == nil {
			t.Fatal("blocked cache path was accepted")
		}
	})
	t.Run("startup", func(t *testing.T) {
		manager := startManager(t, context.Background())
		manager.err = errInjected
		if _, err := manager.Start("viewer", item, "original"); !errors.Is(err, errInjected) {
			t.Fatalf("startup error = %v", err)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		manager := startManager(t, ctx)
		if _, err := manager.Start("viewer", item, "original"); !errors.Is(err, context.Canceled) {
			t.Fatalf("context error = %v", err)
		}
	})
}

func TestStartRejectsExistingOwnerCollision(t *testing.T) {
	manager := startManager(t, context.Background())
	item := downloadItem()
	job, err := manager.Start("viewer", item, "original")
	if err != nil {
		t.Fatal(err)
	}
	existing := job
	existing.Profile = "other"
	manager.jobs[job.ID] = existing
	returned, err := manager.Start("viewer", item, "original")
	if err == nil || returned.Profile != "" {
		t.Fatalf("Start() = %#v, %v", returned, err)
	}
}

func TestStartRollsBackWhenContextStopsAfterPersistence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	manager := startManager(t, ctx)
	manager.tasks = make(chan task)
	manager.persist = func(path string, value any) error {
		if err := persistJSON(path, value); err != nil {
			return err
		}
		cancel()
		return nil
	}
	item := downloadItem()
	job, _ := newJob(manager.root, "viewer", item, "original")
	if _, err := manager.Start("viewer", item, "original"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() error = %v", err)
	}
	if _, found := manager.Get("viewer", job.ID); found || len(manager.pending) != 0 {
		t.Fatal("canceled queue handoff retained state")
	}
	if _, err := os.Stat(filepath.Join(manager.root, job.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("canceled state file remains: %v", err)
	}
}

func TestPrepareReportsFinalPersistenceFailure(t *testing.T) {
	media := filepath.Join(t.TempDir(), "film.mkv")
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := startManager(t, context.Background())
	manager.persist = func(string, any) error { return errInjected }
	item := downloadItem()
	item.Path = media
	job, err := newJob(manager.root, "viewer", item, "original")
	if err != nil {
		t.Fatal(err)
	}
	manager.jobs[job.ID] = job
	manager.prepare(job, item)
	failed, found := manager.Get("viewer", job.ID)
	if !found || failed.State != "failed" || failed.ReadyOffline || failed.Error != "download state could not be saved" {
		t.Fatalf("failed preparation = %#v, %v", failed, found)
	}
	if _, err := os.Stat(job.File); !os.IsNotExist(err) {
		t.Fatalf("uncommitted media remains: %v", err)
	}
}

func TestManagerDefaultsAndSaveValidation(t *testing.T) {
	manager := &Manager{}
	if settings := manager.settings(false); settings.Accelerator != "" || settings.Encoder != "" {
		t.Fatalf("default settings = %#v", settings)
	}
	if err := manager.save(Job{}); err != nil {
		t.Fatalf("disabled persistence = %v", err)
	}
	manager.root = t.TempDir()
	if err := manager.save(Job{}); err == nil {
		t.Fatal("invalid state was persisted")
	}
	job := validStateJob()
	if err := manager.save(job); err == nil {
		t.Fatal("missing persistence was accepted")
	}
}

func startManager(t *testing.T, ctx context.Context) *Manager {
	t.Helper()
	return &Manager{
		ctx: ctx, root: t.TempDir(), persist: persistJSON, jobs: make(map[string]Job),
		pending: make(chan struct{}, QueueCapacity+1), tasks: make(chan task, QueueCapacity),
	}
}

func downloadItem() library.Item {
	return library.Item{ID: "film", Kind: "video", Title: "Film", Path: "/media/film.mkv", Added: time.Now()}
}
