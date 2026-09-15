package downloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestDownloadsReserveExactlyOneSlot(t *testing.T) { //nolint:cyclop,gocognit // Each lifecycle case counts acquisition and release on the same reservation.
	for _, quality := range []string{"original", "compatible", "720p", "audio"} {
		t.Run(quality, func(t *testing.T) {
			// Bound a double-reservation deadlock without treating loaded-host disk latency as a failure.
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			root := t.TempDir()
			media, executable := filepath.Join(root, "film.mp4"), filepath.Join(root, "ffmpeg")
			if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(executable, []byte("#!/bin/sh\nfor output; do :; done\nprintf media > \"$output\"\n"), 0o700); err != nil { //nolint:gosec // This private temporary fixture must be executable by the test.
				t.Fatal(err)
			}
			governor := workload.New(1)
			var generic, encoding atomic.Int32
			manager := New(Config{
				Context: ctx, Cache: root, FFmpeg: executable, Persist: persistJSON,
				Transcoding: func(bool) transcodepolicy.Settings {
					return transcodepolicy.Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264"}
				},
				Inspect: func(context.Context, library.Item) playback.MediaFacts {
					return playback.MediaFacts{Video: playback.VideoFacts{Codec: "h264", Width: 640, Height: 360, BitDepth: 8, FrameRate: 24}}
				},
				Acquire: func(ctx context.Context) (func(), error) {
					generic.Add(1)
					return governor.Acquire(ctx, workload.Background)
				},
				AcquireEncoding: func(ctx context.Context, _ transcodepolicy.Settings) (func(), error) {
					encoding.Add(1)
					return governor.AcquireEncoding(ctx, workload.Background, 1, "gpu")
				},
			})
			item := library.Item{ID: "film", Title: "Film", Path: media, Kind: "video", Added: time.Now()}
			if quality == "audio" {
				item.Kind = "audio"
			}
			job, err := manager.Start("viewer", item, quality)
			if err != nil {
				t.Fatal(err)
			}
			ready, err := manager.Wait(ctx, "viewer", job.ID)
			if err != nil || !ready.ReadyOffline {
				current, _ := manager.Get("viewer", job.ID)
				t.Fatalf("download did not finish with one slot: %#v, %v; current=%#v metrics=%#v", ready, err, current, governor.Metrics())
			}
			wantEncoding := int32(0)
			if quality == "720p" {
				wantEncoding = 1
			}
			if encoding.Load() != wantEncoding || generic.Load() != 1-wantEncoding || governor.Metrics().ActiveBackground != 0 {
				t.Fatalf("reservations: generic=%d encoding=%d metrics=%#v", generic.Load(), encoding.Load(), governor.Metrics())
			}
		})
	}
}

func TestReservationFailureDoesNotRetryOrEncode(t *testing.T) {
	manager := startManager(t, t.Context())
	manager.ffmpeg = "must-not-run"
	manager.transcoding = func(software bool) transcodepolicy.Settings {
		if software {
			t.Fatal("reservation failure authorized an encoding retry")
		}
		return transcodepolicy.Settings{Codec: "h264", Encoder: "h264_nvenc", Accelerator: "cuda"}
	}
	manager.inspect = func(context.Context, library.Item) playback.MediaFacts {
		return playback.MediaFacts{Video: playback.VideoFacts{Codec: "h264", Width: 640, Height: 360}}
	}
	manager.acquireEncoding = func(context.Context, transcodepolicy.Settings) (func(), error) {
		return nil, errors.New("reservation failed")
	}
	item := downloadItem()
	job, err := newJob(manager.root, "viewer", item, "720p")
	if err != nil {
		t.Fatal(err)
	}
	manager.jobs[job.ID] = job
	manager.prepare(job, item)
	failed, _ := manager.Get("viewer", job.ID)
	if failed.State != "failed" || failed.Error != "reservation failed" {
		t.Fatalf("reservation failure = %#v", failed)
	}
	if _, err := os.Stat(job.File); !os.IsNotExist(err) {
		t.Fatalf("failed reservation created output: %v", err)
	}
}

func TestInvalidDownloadIsRejectedBeforeInspection(t *testing.T) {
	manager := startManager(t, t.Context())
	manager.inspect = func(context.Context, library.Item) playback.MediaFacts {
		t.Fatal("invalid input launched media inspection")
		return playback.MediaFacts{}
	}
	for _, quality := range []string{"", "unknown", "audio"} {
		if _, err := manager.Start("viewer", downloadItem(), quality); err == nil {
			t.Fatalf("accepted invalid quality %q", quality)
		}
	}
	if _, err := manager.Start("", downloadItem(), "720p"); err == nil {
		t.Fatal("accepted missing profile")
	}
	if len(manager.jobs) != 0 || len(manager.pending) != 0 {
		t.Fatal("invalid input queued work")
	}
}
