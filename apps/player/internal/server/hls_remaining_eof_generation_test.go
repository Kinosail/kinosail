package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// These isolated controls cover source/route, cancellation and replacement paths
// absent from the public ten-second fixture; fake assets do not prove AAC content.
func TestRemainingColdAACCompletionRejectsGenerationChangedBeforeJoin(t *testing.T) {
	for _, changed := range []string{"root", "rendition"} {
		t.Run(changed, func(t *testing.T) {
			manager, item, recipe, directory := remainingAACAdmissionFixture(t)
			key := hlsRecipeKey(item.ID, recipe)
			options, err := manager.hlsSettings(item, recipe)
			if err != nil {
				t.Fatal(err)
			}
			generation, err := manager.remainingColdAACGeneration(key)
			if err != nil {
				t.Fatal(err)
			}
			job := &hlsJob{done: make(chan struct{}), cachePolicy: options.Cache}
			manager.jobs[key] = job
			target := directory
			if changed == "rendition" {
				target = filepath.Join(directory, "audio")
			}
			if err := os.Rename(target, target+".before-join"); err != nil {
				t.Fatal(err)
			}
			cloneRemainingColdFixture(t, target+".before-join", target)
			manifest := remainingInitialAACPrefix + "#EXTINF:0.021333,\nsegment-00005.m4s\n#EXT-X-ENDLIST\n"
			writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), manifest)
			for n := range 6 {
				writeHLSLoadingFile(t, filepath.Join(directory, "audio", fmtRemainingColdSegment(n)), "synthetic-fragment")
			}
			close(job.done)
			if _, err := manager.remainingColdAACComplete(t.Context(), item, recipe, key, options.Cache, job, 10, generation); err == nil {
				t.Fatal("replacement generation inherited the old worker's completion")
			}
		})
	}
}

func TestRemainingColdAACRetainedRootCannotHideCanonicalReplacement(t *testing.T) {
	for _, changed := range []string{"root", "rendition"} {
		t.Run(changed, func(t *testing.T) {
			manager, _, _, directory := remainingAACAdmissionFixture(t)
			key := filepath.Base(directory)
			bound, err := manager.remainingColdAACGeneration(key)
			if err != nil {
				t.Fatal(err)
			}
			held, err := manager.openCopiedHLSRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Close()
			target := directory
			if changed == "rendition" {
				target = filepath.Join(directory, "audio")
			}
			if err := os.Rename(target, target+".during-read"); err != nil {
				t.Fatal(err)
			}
			cloneRemainingColdFixture(t, target+".during-read", target)
			old, err := held.Lstat(".")
			if err != nil || !os.SameFile(bound.files["."], old) {
				t.Fatal("retained root no longer demonstrates the original gap")
			}
			if manager.remainingColdAACVerifyGeneration(key, bound) == nil {
				t.Fatal("retained directory hid a canonical replacement")
			}
		})
	}
}

func waitRemainingColdAACReaders(t *testing.T, count int) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	buffer := make([]byte, 1<<20)
	for {
		size := runtime.Stack(buffer, true)
		if size == len(buffer) {
			t.Fatal("reader wait witness exceeded its stack bound")
		}
		waiting := 0
		for _, stack := range strings.Split(string(buffer[:size]), "\n\n") {
			if strings.Contains(stack, "remainingColdAACWait(") && strings.Contains(stack, "remainingColdAACProjection(") && strings.Contains(stack, "TestRemainingColdAACConcurrentReadersShareJoinedPublication.func") {
				waiting++
			}
		}
		if waiting >= count {
			return
		}
		select {
		case <-timer.C:
			t.Fatal("concurrent readers did not enter the shared completion wait")
		case <-ticker.C:
		}
	}
}
