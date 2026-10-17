//go:build linux

package playback

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestManyHLSWaitersShareOneInotifyInstanceAndWakePromptly(t *testing.T) {
	const directories, waiters = 50, 300
	root := t.TempDir()
	baseline := hlsWaitBaseline(t)
	paths := make([]string, directories)
	for index := range paths {
		paths[index] = filepath.Join(root, fmt.Sprintf("job-%02d", index), "360p")
		parent := paths[index]
		if index%2 == 1 {
			parent = filepath.Dir(parent)
		}
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	var probes [waiters]atomic.Int64
	returned := make([]chan time.Time, waiters)
	for index := range waiters {
		returned[index] = make(chan time.Time, 1)
		directory, target := paths[index%directories], hlsWaiterTarget(paths, index)
		go func() {
			ready := WaitHLSChange(t.Context(), []string{directory}, nil, func() error {
				probes[index].Add(1)
				_, err := os.Stat(target)
				return err
			})
			if !ready {
				t.Error("waiter did not observe its segment")
			}
			returned[index] <- time.Now()
		}()
	}
	for start := time.Now(); !allHLSWaitersSubscribed(probes[:]); time.Sleep(time.Millisecond) {
		if time.Since(start) > 5*time.Second {
			t.Fatal("waiters did not start")
		}
	}
	if instances := inotifyInstances(t); instances != 1 {
		t.Fatalf("%d waiters use %d inotify instances", waiters, instances)
	}
	for index := 1; index < directories; index += 2 {
		createHLSTestDirectory(t, paths[index])
	}
	time.Sleep(5 * time.Millisecond)
	var latencies []time.Duration
	for index := range waiters {
		published := publishHLSTestFile(t, hlsWaiterTarget(paths, index), "segment")
		select {
		case at := <-returned[index]:
			latencies = append(latencies, at.Sub(published))
		case <-time.After(2 * time.Second):
			t.Fatalf("waiter %d did not wake", index)
		}
	}
	if median := medianDuration(latencies); median > promptHLSWake {
		t.Fatalf("median wake across %d waiters = %v", waiters, median)
	}
	assertHLSWatchesReleased(t, root)
	if instances := inotifyInstances(t); instances != 1 {
		t.Fatalf("inotify instances after waits = %d", instances)
	}
	for start := time.Now(); runtime.NumGoroutine() > baseline; time.Sleep(10 * time.Millisecond) {
		if time.Since(start) > 2*time.Second {
			t.Fatalf("goroutines = %d, baseline = %d", runtime.NumGoroutine(), baseline)
		}
	}
}

func TestHLSWaitersRewatchARemovedAndRecreatedDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "job", "360p")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	early := filepath.Join(directory, "segment-00001.m4s")
	stale := make(chan bool, 1)
	go func() {
		stale <- WaitHLSChange(t.Context(), []string{directory}, nil, func() error { _, err := os.Stat(early); return err })
	}()
	time.Sleep(10 * time.Millisecond)
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	createHLSTestDirectory(t, directory)
	time.Sleep(5 * time.Millisecond)
	var latencies []time.Duration
	for trial := range 8 {
		target := filepath.Join(directory, fmt.Sprintf("segment-%05d.m4s", trial+10))
		returned := make(chan time.Time, 1)
		go func() {
			WaitHLSChange(t.Context(), []string{directory}, nil, func() error { _, err := os.Stat(target); return err })
			returned <- time.Now()
		}()
		time.Sleep(hlsWakeDelay(trial))
		published := publishHLSTestFile(t, target, "segment")
		select {
		case at := <-returned:
			latencies = append(latencies, at.Sub(published))
		case <-time.After(2 * time.Second):
			t.Fatal("waiter on the recreated directory did not wake")
		}
	}
	if median := medianDuration(latencies); median > promptHLSWake {
		t.Fatalf("median wake after the directory was recreated = %v, latencies = %v", median, latencies)
	}
	publishHLSTestFile(t, early, "segment")
	select {
	case ready := <-stale:
		if !ready {
			t.Fatal("waiter on the removed directory did not observe its segment")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter on the removed directory did not wake")
	}
	assertHLSWatchesReleased(t, filepath.Dir(directory))
}

func hlsWaiterTarget(paths []string, index int) string {
	return filepath.Join(paths[index%len(paths)], fmt.Sprintf("segment-%05d.m4s", index))
}

func allHLSWaitersSubscribed(probes []atomic.Int64) bool {
	for index := range probes {
		if probes[index].Load() < 2 {
			return false
		}
	}
	return true
}

func inotifyInstances(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skip("process file descriptors cannot be listed")
	}
	count := 0
	for _, entry := range entries {
		if target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name())); err == nil && target == "anon_inode:inotify" {
			count++
		}
	}
	return count
}
