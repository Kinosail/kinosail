package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHLSSkipInputBurstsBeforeSettlingToSourceRate(t *testing.T) {
	arguments, rate, err := hlsSkipInput(nil, t.TempDir(), "/media/movie.mp4", 60, hlsRecipe{})
	if err != nil {
		t.Fatal(err)
	}
	if rate != "1" || !strings.Contains(strings.Join(arguments, " "), "-readrate_initial_burst 16 -readrate 1 -f concat") {
		t.Fatalf("skip input pacing = %q, %q", arguments, rate)
	}
}

func TestHLSJobIdleTimeoutResetsOnSegmentActivity(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	job := &hlsJob{cancel: cancel, activity: make(chan struct{}, 1)}
	go watchHLSJob(ctx, job, 100*time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	job.activity <- struct{}{}
	select {
	case <-ctx.Done():
		t.Fatalf("active HLS job canceled early: %v", context.Cause(ctx))
	case <-time.After(60 * time.Millisecond):
	}
	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), errHLSInactive) {
			t.Fatalf("idle cancellation = %v", context.Cause(ctx))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("idle HLS job remained active")
	}
}

func TestStoppedPlaybackCancelsOnlyItsHLSJobs(t *testing.T) {
	firstContext, firstCancel := context.WithCancelCause(t.Context())
	secondContext, secondCancel := context.WithCancelCause(t.Context())
	defer secondCancel(nil)
	manager := &hlsManager{jobs: map[string]*hlsJob{
		"first":  {cancel: firstCancel, playbackSession: "aaaaaaaa"},
		"second": {cancel: secondCancel, playbackSession: "bbbbbbbb"},
	}}
	manager.stopHLSSession("aaaaaaaa")
	select {
	case <-firstContext.Done():
		if !errors.Is(context.Cause(firstContext), errHLSInactive) {
			t.Fatalf("stopped cancellation = %v", context.Cause(firstContext))
		}
	case <-time.After(time.Second):
		t.Fatal("stopped playback kept its HLS job")
	}
	select {
	case <-secondContext.Done():
		t.Fatal("stopping one playback canceled another")
	default:
	}
	manager.stopHLSSession("invalid session")
	select {
	case <-secondContext.Done():
		t.Fatal("invalid playback session canceled an HLS job")
	default:
	}
}

func TestPlaybackProgressKeepsOnlyItsHLSJobsAlive(t *testing.T) {
	first := &hlsJob{activity: make(chan struct{}, 1), playbackSession: "aaaaaaaa"}
	second := &hlsJob{activity: make(chan struct{}, 1), playbackSession: "bbbbbbbb"}
	manager := &hlsManager{jobs: map[string]*hlsJob{"first": first, "second": second}}
	manager.keepHLSSessionAlive("aaaaaaaa")
	select {
	case <-first.activity:
	default:
		t.Fatal("playback progress did not keep its HLS job active")
	}
	select {
	case <-second.activity:
		t.Fatal("playback progress kept another HLS job active")
	default:
	}
	manager.keepHLSSessionAlive("invalid session")
	select {
	case <-second.activity:
		t.Fatal("invalid playback progress kept an HLS job active")
	default:
	}
}

func TestActiveSeekOnlyCoversItsForwardSegmentWindow(t *testing.T) {
	rendering := filepath.Join(t.TempDir(), "1080p")
	if err := os.MkdirAll(rendering, 0o700); err != nil {
		t.Fatal(err)
	}
	job := &hlsJob{startNumber: 75}
	for segment, covered := range map[int]bool{3: false, 75: true, 76: true, 77: true, 78: false} {
		if got := job.coversSegment(rendering, segment); got != covered {
			t.Errorf("segment %d covered = %t, want %t", segment, got, covered)
		}
	}
	seek := filepath.Join(filepath.Dir(rendering), hlsSeekDirectory(75), filepath.Base(rendering))
	if err := os.MkdirAll(seek, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seek, "index.m3u8"), []byte("#EXTM3U\nsegment-00075.m4s\nsegment-00076.m4s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !job.coversSegment(rendering, 78) || job.coversSegment(rendering, 79) {
		t.Fatal("active seek did not follow its published forward window")
	}
}
