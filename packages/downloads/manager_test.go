package downloads

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestOriginalDownloadLifecycle(t *testing.T) { //nolint:cyclop,gocognit // One test follows the public lifecycle end to end.
	t.Parallel()
	cache := t.TempDir()
	media := filepath.Join(t.TempDir(), "Film.mkv")
	if err := os.WriteFile(media, []byte("original-media"), 0o600); err != nil {
		t.Fatal(err)
	}
	events := make(chan Job, 3)
	manager := New(Config{Context: t.Context(), Cache: cache, Persist: persistJSON, Publish: func(job Job) { events <- job }})
	item := library.Item{ID: "film", Kind: "video", Title: "Film", Path: media, Added: time.Now()}
	started, err := manager.Start("viewer", item, "original")
	if err != nil || started.State != "preparing" {
		t.Fatalf("start = %#v, %v", started, err)
	}
	repeated, err := manager.Start("viewer", item, "original")
	if err != nil || repeated.ID != started.ID {
		t.Fatalf("idempotent start = %#v, %v", repeated, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ready, err := manager.Wait(ctx, "viewer", started.ID)
	if err != nil || !ready.ReadyOffline || ready.SHA256 == "" || ready.Size != 14 {
		t.Fatalf("ready = %#v, %v", ready, err)
	}
	if data, readErr := os.ReadFile(ready.File); readErr != nil || string(data) != "original-media" {
		t.Fatalf("stored data = %q, %v", data, readErr)
	}
	if listed := manager.List("viewer"); len(listed) != 1 || listed[0].ID != ready.ID {
		t.Fatalf("listed = %#v", listed)
	}
	if _, found := manager.Get("other", ready.ID); found {
		t.Fatal("another profile accessed the job")
	}
	if err := manager.Remove("viewer", ready.ID); err != nil || len(manager.jobs) != 0 {
		t.Fatalf("remove = %v, jobs = %d", err, len(manager.jobs))
	}
	for range 2 {
		select {
		case <-events:
		case <-ctx.Done():
			t.Fatal("download event was not published")
		}
	}
}

func TestStartRejectsInvalidOrUnavailableRequests(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "film", Kind: "video", Title: "Film", Path: "/media/film.mp4", Added: time.Now()}
	if _, err := New(Config{}).Start("viewer", item, "original"); err == nil {
		t.Fatal("unconfigured cache was accepted")
	}
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Persist: persistJSON})
	for _, test := range []struct {
		profile, quality, kind string
	}{
		{"viewer", "bad", "video"},
		{"viewer", "audio", "video"},
		{"viewer", "720p", "photo"},
		{"", "original", "video"},
	} {
		candidate := item
		candidate.Kind = test.kind
		if _, err := manager.Start(test.profile, candidate, test.quality); err == nil {
			t.Fatalf("invalid request was accepted: %#v", test)
		}
	}
}

func TestStartPreservesMemoryWhenPersistenceFails(t *testing.T) {
	t.Parallel()
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Persist: func(string, any) error { return errors.New("blocked") }})
	item := library.Item{ID: "film", Kind: "video", Title: "Film", Path: "/media/film.mp4", Added: time.Now()}
	if _, err := manager.Start("viewer", item, "original"); err == nil {
		t.Fatal("download start succeeded")
	}
	if len(manager.jobs) != 0 || len(manager.pending) != 0 {
		t.Fatalf("failed start changed state: jobs=%d pending=%d", len(manager.jobs), len(manager.pending))
	}
}

func TestQueueRejectsOverflowBeforeCreatingAJob(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	manager := New(Config{
		Context: ctx, Cache: t.TempDir(), FFmpeg: "unused", Persist: persistJSON,
		Acquire: func(wait context.Context) (func(), error) {
			<-wait.Done()
			return nil, wait.Err()
		},
	})
	rejected := 0
	for position := range QueueCapacity + 2 {
		item := library.Item{ID: string(rune('a' + position)), Kind: "audio", Title: "Track", Path: "/missing", Added: time.Now()}
		if _, err := manager.Start("viewer", item, "audio"); err != nil {
			rejected++
		}
	}
	if rejected != 1 || len(manager.jobs) != QueueCapacity+1 {
		t.Fatalf("rejected=%d jobs=%d", rejected, len(manager.jobs))
	}
	cancel()
	for deadline := time.Now().Add(3 * time.Second); len(manager.pending) != 0 && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
	}
}

func TestTranscodeRetriesWithSoftware(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	count, executable := filepath.Join(directory, "count"), filepath.Join(directory, "ffmpeg")
	script := "#!/bin/sh\ncount=0; [ ! -f '" + count + "' ] || count=$(cat '" + count + "'); count=$((count+1)); echo $count > '" + count + "'; [ $count -gt 1 ] || exit 1; for output; do :; done; printf media > \"$output\"\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil { //nolint:gosec // Test fixture must be executable.
		t.Fatal(err)
	}
	var software atomic.Bool
	manager := &Manager{
		ctx: t.Context(), root: t.TempDir(), ffmpeg: executable, persist: persistJSON, jobs: make(map[string]Job),
		transcoding: func(fallback bool) transcodepolicy.Settings {
			software.Store(fallback)
			return transcodepolicy.Settings{Codec: "h264", Accelerator: map[bool]string{false: "vaapi", true: "none"}[fallback]}
		},
	}
	item := library.Item{ID: "track", Kind: "audio", Title: "Track", Path: "/media/track.flac", Added: time.Now()}
	job, err := newJob(manager.root, "viewer", item, "audio")
	if err != nil {
		t.Fatal(err)
	}
	manager.jobs[job.ID] = job
	manager.prepareTask(task{job: job, item: item})
	ready, found := manager.Get("viewer", job.ID)
	if !found || ready.State != "ready" || !software.Load() {
		t.Fatalf("software fallback = %v, job = %#v", software.Load(), ready)
	}
}

func TestEncodeBuildsVideoCommandAndReportsCopyErrors(t *testing.T) {
	t.Parallel()
	manager := &Manager{
		ctx: t.Context(), ffmpeg: "/usr/bin/true",
		inspect: func(context.Context, library.Item) playback.MediaFacts {
			return playback.MediaFacts{Video: playback.VideoFacts{Codec: "h264", Width: 1280, Height: 720}}
		},
		transcoding: func(bool) transcodepolicy.Settings {
			return transcodepolicy.Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264"}
		},
	}
	if err := manager.encodeSelectedContext(t.Context(), library.Item{Kind: "video", Path: "/media/film.mkv"}, "720p", "/cache/output.mp4", false, nil); err != nil {
		t.Fatalf("video command = %v", err)
	}
	if err := copyFileContext(t.Context(), "/missing", filepath.Join(t.TempDir(), "output")); !os.IsNotExist(err) {
		t.Fatalf("missing input error = %v", err)
	}
}

func persistJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600) //nolint:gosec // Tests persist only manager-derived paths under TempDir.
}
