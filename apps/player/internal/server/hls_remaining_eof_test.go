package server

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sync"
	"testing"
	"time"
)

// These isolated controls cover source/route, cancellation and replacement paths
// absent from the public ten-second fixture; fake assets do not prove AAC content.
func TestRemainingColdAACEligibilityCannotBroadenCompletionWait(t *testing.T) {
	facts := MediaFacts{Kind: "audio", Container: "flac", Duration: 10,
		Audio: []AudioFacts{{Index: 0, SourceIndex: 0, Codec: "flac", SampleRate: 48000, Channels: 2, ChannelLayout: "stereo"}}}
	recipe := hlsRecipe{mode: "audio-transcode"}
	if !remainingColdAACEligible(facts, recipe, "audio/index.m3u8", http.MethodGet, 10) {
		t.Fatal("qualified short source did not retain its complete timeline")
	}
	edits := []struct {
		name string
		edit func(*MediaFacts, *hlsRecipe, *string, *string, *float64)
	}{
		{"long-source", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, d *float64) { f.Duration, *d = 120, 120 }},
		{"over-qualified-bound", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, d *float64) { f.Duration, *d = 10.000001, 10.000001 }},
		{"short-prefix-source", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, d *float64) { f.Duration, *d = 8, 8 }},
		{"nan-duration", func(_ *MediaFacts, _ *hlsRecipe, _, _ *string, d *float64) { *d = math.NaN() }},
		{"infinite-duration", func(_ *MediaFacts, _ *hlsRecipe, _, _ *string, d *float64) { *d = math.Inf(1) }},
		{"duration-mismatch", func(_ *MediaFacts, _ *hlsRecipe, _, _ *string, d *float64) { *d = 9 }},
		{"head", func(_ *MediaFacts, _ *hlsRecipe, _ *string, m *string, _ *float64) { *m = http.MethodHead }},
		{"master", func(_ *MediaFacts, _ *hlsRecipe, n, _ *string, _ *float64) { *n = "index.m3u8" }},
		{"segment", func(_ *MediaFacts, _ *hlsRecipe, n, _ *string, _ *float64) { *n = "audio/segment-00000.m4s" }},
		{"wrong-mode", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) { r.mode = "remux" }},
		{"seek", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) { r.offset = 8 }},
		{"output-offset", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) { r.outputTime = 8 }},
		{"alternate-track", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) { r.audio = 1 }},
		{"lower-rate", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) { r.maxBitrate = 128000 }},
		{"boost", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) { r.dialogueBoost = true }},
		{"normalize", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) { r.normalizeLoudness = true }},
		{"omitted", func(_ *MediaFacts, r *hlsRecipe, _, _ *string, _ *float64) {
			r.omitted = []PlaybackRange{{Start: 1, End: 2}}
		}},
		{"video", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, _ *float64) { f.Video.Codec = "h264" }},
		{"wrong-container", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, _ *float64) { f.Container = "mp3" }},
		{"wrong-codec", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, _ *float64) { f.Audio[0].Codec = "aac" }},
		{"wrong-rate", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, _ *float64) { f.Audio[0].SampleRate = 44100 }},
		{"wrong-layout", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, _ *float64) { f.Audio[0].ChannelLayout = "5.1" }},
		{"multiple-tracks", func(f *MediaFacts, _ *hlsRecipe, _, _ *string, _ *float64) { f.Audio = append(f.Audio, f.Audio[0]) }},
	}
	for _, sample := range edits {
		t.Run(sample.name, func(t *testing.T) {
			f, r, n, m, d := facts, recipe, "audio/index.m3u8", http.MethodGet, 10.0
			f.Audio = append([]AudioFacts(nil), facts.Audio...)
			sample.edit(&f, &r, &n, &m, &d)
			if remainingColdAACEligible(f, r, n, m, d) {
				t.Fatal("ineligible route acquired a completion wait")
			}
		})
	}
}

func TestRemainingColdAACDoesNotWaitForPreparedOrRefillJobs(t *testing.T) {
	for _, job := range []*hlsJob{nil,
		{done: make(chan struct{}), preparation: &startupEncoding{}},
		{done: make(chan struct{}), startNumber: 4},
	} {
		if wait, err := remainingColdAACInitial(job, "policy"); wait || err != nil {
			t.Fatalf("prepared/refill path changed: wait=%t err=%v", wait, err)
		}
	}
	for _, job := range []*hlsJob{
		{done: make(chan struct{}), cachePolicy: "different"},
		{done: make(chan struct{}), cachePolicy: "policy", replacing: true},
		{cachePolicy: "policy"},
	} {
		if wait, err := remainingColdAACInitial(job, "policy"); wait || err == nil {
			t.Fatal("invalid initial publication was admitted")
		}
	}
}

func TestRemainingColdAACWaitJoinsWithoutLifecycleMutation(t *testing.T) {
	done, activity := make(chan struct{}), make(chan struct{}, 1)
	cancelled := false
	job := &hlsJob{done: done, activity: activity, cancel: func(error) { cancelled = true }}
	var readers sync.WaitGroup
	for range 2 {
		readers.Go(func() {
			if err := remainingColdAACWait(t.Context(), job); err != nil {
				t.Error(err)
			}
		})
	}
	close(done)
	readers.Wait()
	if cancelled || len(activity) != 0 || job.preparation != nil {
		t.Fatal("wait changed encoder lifecycle")
	}
	failure := errors.New("synthetic-publication-failure")
	job.err = failure
	if !errors.Is(remainingColdAACWait(t.Context(), job), failure) {
		t.Fatal("publication failure disappeared")
	}
}

func TestRemainingColdAACWaitReleasesOnCancellationAndTimeout(t *testing.T) {
	job := &hlsJob{done: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if !errors.Is(remainingColdAACWait(ctx, job), context.Canceled) {
		t.Fatal("cancelled request kept waiting")
	}
	started := time.Now()
	if !errors.Is(remainingColdAACWait(t.Context(), job), context.DeadlineExceeded) ||
		time.Since(started) > 8*time.Second {
		t.Fatal("cold completion wait was not bounded")
	}
	select {
	case <-job.done:
		t.Fatal("request wait cancelled the shared worker")
	default:
	}
}
