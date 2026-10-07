package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

// These real filesystem/process races cannot be scheduled through public E2E.
func copiedRecoveryRefill(t *testing.T) (*hlsManager, library.Item, hlsRecipe, string) {
	t.Helper()
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, "360p/segment-00000.m4s")); err != nil {
		t.Fatal(err)
	}
	return manager, item, recipe, directory
}

func copiedRecoveryEncoder(t *testing.T, manager *hlsManager, action string) {
	copiedRecoveryEncoderOutput(t, manager, action, "initialization", "first fragment", copiedRecoveryManifest)
}

func copiedRecoveryEncoderOutput(t *testing.T, manager *hlsManager, action, init, first, manifest string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "owned-encoder")
	body := "#!/bin/sh\nset -eu\nsegments=''\nplaylist=''\nwhile [ \"$#\" -gt 0 ]; do\n case \"$1\" in\n -hls_segment_filename) shift; segments=$1 ;;\n esac\n playlist=$1\n shift\ndone\ndirectory=${segments%/*}\n" + action + "\n"
	body += mp4fixture.Shell([]byte(init)) + " > \"$directory/init.mp4\"\nprintf '%s' " + copiedRecoveryQuote(first) + " > \"$directory/segment-00000.m4s\"\nprintf 'last fragment' > \"$directory/segment-00001.m4s.tmp\"\nmv \"$directory/segment-00001.m4s.tmp\" \"$directory/segment-00001.m4s\"\nprintf '%s' " + copiedRecoveryQuote(manifest) + " > \"$playlist\"\n"
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil { //nolint:gosec // Owned controlled executable protects a filesystem race, never media output proof.
		t.Fatal(err)
	}
	manager.ffmpeg = path
}

func copiedRecoveryRunRefill(ctx context.Context, manager *hlsManager, item library.Item, recipe hlsRecipe, directory string) error {
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return err
	}
	return manager.encodeVariant(ctx, item, directory, "360p", "640", "1000k", "128k", 4, options, recipe, recipe, 0, 0)
}

func copiedRecoveryPreserved(t *testing.T, directory string) func() {
	t.Helper()
	names := []string{"index.m3u8", ".source", ".copy-timeline", ".copy-clock", "360p/index.m3u8", "360p/init.mp4", "360p/segment-00001.m4s"}
	data := make([][]byte, len(names))
	info := make([]os.FileInfo, len(names))
	for number, name := range names {
		var err error
		data[number], err = os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		info[number], err = os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
	}
	return func() {
		t.Helper()
		for number, name := range names {
			after, err := os.ReadFile(filepath.Join(directory, name))
			stat, statErr := os.Stat(filepath.Join(directory, name))
			if err != nil || statErr != nil || !bytes.Equal(data[number], after) || !sameCopiedHLSFile(info[number], stat) {
				t.Errorf("refill replaced committed %s bytes or identity", name)
			}
		}
	}
}

func TestCopiedRecoveryRefillPreservesCommittedGeneration(t *testing.T) {
	manager, item, recipe, directory := copiedRecoveryRefill(t)
	check := copiedRecoveryPreserved(t, directory)
	copiedRecoveryEncoder(t, manager, "")
	if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err != nil {
		t.Fatal(err)
	}
	check()
	data, err := os.ReadFile(filepath.Join(directory, "360p/segment-00000.m4s"))
	if err != nil || string(data) != "first fragment" {
		t.Fatal("certified first fragment was not published")
	}
}

func TestCopiedRecoveryRefillRejectsChangedMetadata(t *testing.T) {
	for _, name := range []string{".source", ".copy-timeline", ".copy-clock", "360p/index.m3u8", "360p/init.mp4"} {
		t.Run(name, func(t *testing.T) {
			manager, item, recipe, directory := copiedRecoveryRefill(t)
			copiedRecoveryEncoder(t, manager, "printf damaged > "+copiedRecoveryQuote(filepath.Join(directory, name)))
			if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err == nil {
				t.Fatal("changed committed metadata acquired regenerated media")
			}
			if _, err := os.Lstat(filepath.Join(directory, "360p/segment-00000.m4s")); !os.IsNotExist(err) {
				t.Fatal("stale media was published after metadata changed")
			}
		})
	}
}

func TestCopiedRecoveryRefillRejectsReplacementGeneration(t *testing.T) {
	for _, rendition := range []bool{false, true} {
		t.Run(strconv.FormatBool(rendition), func(t *testing.T) {
			manager, item, recipe, directory := copiedRecoveryRefill(t)
			path := directory
			if rendition {
				path = filepath.Join(path, "360p")
			}
			copiedRecoveryEncoder(t, manager, "mv "+copiedRecoveryQuote(path)+" "+copiedRecoveryQuote(path+"-retired")+"\ncp -R "+copiedRecoveryQuote(path+"-retired")+" "+copiedRecoveryQuote(path))
			if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err == nil {
				t.Fatal("replacement generation accepted stale output")
			}
			if _, err := os.Lstat(filepath.Join(directory, "360p/segment-00000.m4s")); !os.IsNotExist(err) {
				t.Fatal("old worker published into replacement generation")
			}
		})
	}
}

func TestCopiedRecoveryRefillNeverOverwritesConcurrentDestination(t *testing.T) {
	for _, contents := range []string{"other producer", "first fragment"} {
		t.Run(contents, func(t *testing.T) {
			manager, item, recipe, directory := copiedRecoveryRefill(t)
			marker := filepath.Join(t.TempDir(), "inode")
			first := filepath.Join(directory, "360p/segment-00000.m4s")
			copiedRecoveryEncoder(t, manager, "printf '%s' "+copiedRecoveryQuote(contents)+" > "+copiedRecoveryQuote(first)+"\nls -i "+copiedRecoveryQuote(first)+" | awk '{print $1}' > "+copiedRecoveryQuote(marker))
			err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory)
			if (err == nil) != (contents == "first fragment") {
				t.Error("concurrent destination certificate admission was incorrect")
			}
			data, readErr := os.ReadFile(first)
			if readErr != nil || string(data) != contents {
				t.Fatal("concurrent producer's media was overwritten")
			}
			info, statErr := os.Stat(first)
			inode, markerErr := os.ReadFile(marker)
			if statErr != nil || markerErr != nil || string(bytes.TrimSpace(inode)) != strconv.FormatUint(info.Sys().(*syscall.Stat_t).Ino, 10) {
				t.Fatal("concurrent destination inode was replaced")
			}
		})
	}
}

func TestCopiedRecoveryCanceledRefillJoinsAndPreserves(t *testing.T) {
	manager, item, recipe, directory := copiedRecoveryRefill(t)
	check := copiedRecoveryPreserved(t, directory)
	marker := filepath.Join(t.TempDir(), "pid")
	copiedRecoveryEncoder(t, manager, "printf '%s' $$ > "+copiedRecoveryQuote(marker)+"\nexec sleep 30")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- copiedRecoveryRunRefill(ctx, manager, item, recipe, directory) }()
	deadline := time.Now().Add(time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		data, _ = os.ReadFile(marker)
		if len(data) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled refill reported success")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled refill did not join")
	}
	copiedRecoveryAssertStopped(t, data)
	check()
	if _, err := os.Lstat(filepath.Join(directory, "360p/segment-00000.m4s")); !os.IsNotExist(err) {
		t.Fatal("canceled refill published media")
	}
	stages, err := filepath.Glob(filepath.Join(manager.cache, ".copy-refill-*"))
	if err != nil || len(stages) != 0 {
		t.Fatal("canceled refill left its exclusive operation stage")
	}
	copiedRecoveryEncoder(t, manager, "")
	if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err != nil {
		t.Fatal("canceled refill held encoding admission")
	}
}
