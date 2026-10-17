package playback

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

const promptHLSWake = 5 * time.Millisecond

func TestWaitHLSChangeWakesWhenSegmentIsPublished(t *testing.T) {
	var latencies []time.Duration
	for trial := range 16 {
		latency, _ := measureHLSSegmentWake(t, trial)
		latencies = append(latencies, latency)
	}
	if median := medianDuration(latencies); median > promptHLSWake {
		t.Fatalf("median segment wake = %v, latencies = %v", median, latencies)
	}
}

func TestPublishVariantsObservedSignalsReadyWhenVariantIsPublished(t *testing.T) {
	var latencies []time.Duration
	for trial := range 16 {
		latencies = append(latencies, measureHLSPublicationWake(t, trial))
	}
	if median := medianDuration(latencies); median > promptHLSWake {
		t.Fatalf("median publication wake = %v, latencies = %v", median, latencies)
	}
}

func TestHLSWaitersReturnWithoutLeaksWhenCanceledOrDone(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "movie.mkv")
	writeTestFile(t, source, "source")
	quality := PlaybackQuality{Label: "360p", Width: 640, Height: 360, Bitrate: 493_000}
	baseline := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	restart := errors.New("producer finished")
	results := make(chan error, 24)
	for index := range 8 {
		directory := filepath.Join(root, "job", strings.Repeat("a", index+1))
		go func() {
			results <- PublishVariantsObserved(ctx, source, directory, "ffmpeg", []PlaybackQuality{quality}, make(chan error), 1, false, osWriteFile, nil)
		}()
		go func() {
			target := filepath.Join(directory, "360p", "segment-00001.m4s")
			if WaitHLSChange(ctx, []string{filepath.Dir(target)}, nil, func() error { _, err := os.Stat(target); return err }) {
				results <- errors.New("canceled wait became ready")
				return
			}
			results <- ctx.Err()
		}()
		go func() {
			target := filepath.Join(root, "segment-00001.m4s")
			if WaitHLSChange(t.Context(), []string{root}, done, func() error {
				select {
				case <-done:
					return restart
				default:
					_, err := os.Stat(target)
					return err
				}
			}) {
				results <- errors.New("finished producer wait became ready")
				return
			}
			results <- restart
		}()
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	close(done)
	deadline := time.After(2 * time.Second)
	for range cap(results) {
		select {
		case err := <-results:
			if !errors.Is(err, context.Canceled) && !errors.Is(err, restart) {
				t.Fatal(err)
			}
		case <-deadline:
			t.Fatal("HLS waiter did not return after cancellation")
		}
	}
	for start := time.Now(); runtime.NumGoroutine() > baseline; time.Sleep(10 * time.Millisecond) {
		if time.Since(start) > 2*time.Second {
			t.Fatalf("goroutines = %d, baseline = %d", runtime.NumGoroutine(), baseline)
		}
	}
}

func measureHLSSegmentWake(tb testing.TB, trial int) (time.Duration, int64) {
	tb.Helper()
	createDirectory := trial%2 == 1
	directory := filepath.Join(tb.TempDir(), "job", "360p")
	parent := directory
	if createDirectory {
		parent = filepath.Dir(directory)
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		tb.Fatal(err)
	}
	target := filepath.Join(directory, "segment-00001.m4s")
	var probes atomic.Int64
	returned := make(chan time.Time, 1)
	go func() {
		WaitHLSChange(tb.Context(), []string{directory}, nil, func() error {
			probes.Add(1)
			_, err := os.Stat(target)
			return err
		})
		returned <- time.Now()
	}()
	time.Sleep(hlsWakeDelay(trial))
	if createDirectory {
		createHLSTestDirectory(tb, directory)
	}
	published := publishHLSTestFile(tb, target, strings.Repeat("x", 1000))
	select {
	case at := <-returned:
		return at.Sub(published), probes.Load()
	case <-time.After(2 * time.Second):
		tb.Fatal("segment wait did not observe the published segment")
		return 0, 0
	}
}

func measureHLSPublicationWake(tb testing.TB, trial int) time.Duration {
	tb.Helper()
	createDirectory := trial%2 == 1
	root := tb.TempDir()
	source := filepath.Join(root, "movie.mkv")
	publishHLSTestFile(tb, source, "source")
	directory := filepath.Join(root, "job")
	rendition := filepath.Join(directory, "360p")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		tb.Fatal(err)
	}
	if !createDirectory {
		createHLSTestDirectory(tb, rendition)
	}
	quality := PlaybackQuality{Label: "360p", Width: 640, Height: 360, Bitrate: 493_000}
	results, ready, finished := make(chan error), make(chan time.Time, 1), make(chan error, 1)
	go func() {
		finished <- PublishVariantsObserved(tb.Context(), source, directory, "ffmpeg", []PlaybackQuality{quality}, results, 1, false, osWriteFile, func() { ready <- time.Now() })
	}()
	time.Sleep(hlsWakeDelay(trial))
	if createDirectory {
		createHLSTestDirectory(tb, rendition)
	}
	publishHLSTestFile(tb, filepath.Join(rendition, "init.mp4"), string(mp4fixture.Initialization(640, 360, "h264", "aac", "")))
	publishHLSTestFile(tb, filepath.Join(rendition, "segment-00000.m4s"), strings.Repeat("x", 1000))
	published := publishHLSTestFile(tb, filepath.Join(rendition, "index.m3u8"), "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2.000,\nsegment-00000.m4s\n")
	var latency time.Duration
	select {
	case at := <-ready:
		latency = at.Sub(published)
	case <-time.After(2 * time.Second):
		tb.Fatal("publication did not observe the ready variant")
	}
	results <- nil
	if err := <-finished; err != nil || !pathExists(filepath.Join(directory, "index.m3u8")) {
		tb.Fatalf("publication = %v", err)
	}
	return latency
}

func createHLSTestDirectory(tb testing.TB, directory string) {
	tb.Helper()
	if err := os.Mkdir(directory, 0o700); err != nil {
		tb.Fatal(err)
	}
	time.Sleep(time.Millisecond)
}

func publishHLSTestFile(tb testing.TB, path, content string) time.Time {
	tb.Helper()
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(content), 0o600); err != nil {
		tb.Fatal(err)
	}
	if err := os.Rename(temporary, path); err != nil {
		tb.Fatal(err)
	}
	return time.Now()
}

func hlsWakeDelay(trial int) time.Duration {
	return 2*time.Millisecond + time.Duration(trial*7%25)*time.Millisecond
}

func medianDuration(values []time.Duration) time.Duration {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}
