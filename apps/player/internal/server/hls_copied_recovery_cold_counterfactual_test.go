package server

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/isobmff"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func copiedRecoveryInitializationFixture(t *testing.T, empty, atomic bool) (library.Item, string, string, string) {
	t.Helper()
	_, item, _, directory, initialization := copiedRecoveryEnclosingFixture(t)
	for _, name := range []string{".copy-timeline", ".copy-clock", "index.m3u8"} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
	rendition := filepath.Join(directory, "1080p")
	if empty {
		for _, name := range []string{"index.m3u8", "init.mp4", "segment-00001.m4s"} {
			if err := os.Remove(filepath.Join(rendition, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	pending := filepath.Join(rendition, "init.mp4.pending")
	writeHLSLoadingFile(t, pending, initialization)
	if !atomic {
		writeHLSLoadingFile(t, filepath.Join(rendition, "init.mp4"), "")
	}
	if ready := playback.VariantReady(item.Path, rendition); ready != !empty {
		t.Fatalf("scheduled rendition readiness = %v, want %v", ready, !empty)
	}
	policy, err := os.ReadFile(filepath.Join(directory, ".source"))
	if err != nil {
		t.Fatal(err)
	}
	return item, directory, pending, string(policy)
}

func copiedRecoveryInitializationWorker(ctx context.Context, pending string) (chan struct{}, <-chan error, func()) {
	release := make(chan struct{})
	results, stop := playback.StartHLSWorker(ctx, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
		}
		rendition := filepath.Dir(pending)
		if err := os.Rename(pending, filepath.Join(rendition, "init.mp4")); err != nil {
			return err
		}
		for name, data := range map[string]string{"segment-00000.m4s": "first fragment", "segment-00001.m4s": "last fragment"} {
			if err := os.WriteFile(filepath.Join(rendition, name), []byte(data), 0o600); err != nil {
				return err
			}
		}
		return os.WriteFile(filepath.Join(rendition, "index.m3u8"), []byte(copiedRecoveryManifest), 0o600)
	})
	return release, results, stop
}

func copiedRecoveryInitializationMaster(t *testing.T, directory string, err error, invalid bool) {
	t.Helper()
	master := filepath.Join(directory, "index.m3u8")
	if invalid {
		if !errors.Is(err, isobmff.ErrInvalid) {
			t.Fatalf("truncated retained initialization returned %v, want %v", err, isobmff.ErrInvalid)
		}
		if _, statErr := os.Lstat(master); !os.IsNotExist(statErr) {
			t.Fatal("invalid initialization acquired a master")
		}
		return
	}
	if err != nil {
		t.Fatalf("controlled cold/atomic publication failed: %v", err)
	}
	data, readErr := os.ReadFile(master)
	if readErr != nil || !bytes.Contains(data, []byte("1080p/index.m3u8")) {
		t.Fatal("controlled valid publication failed to advertise its rendition")
	}
}

// Schedule only the initialization-read boundary; this is not media proof or
// a diagnosis of the original intermittent failure, whose error was hidden.
func TestCopiedRecoveryOrdinaryPublicationInitializationCounterfactual(t *testing.T) {
	for _, sample := range []struct {
		name          string
		empty, atomic bool
	}{
		{name: "retained-unindexed-truncated-init"},
		{name: "empty-cold-truncated-init", empty: true},
		{name: "retained-unindexed-atomic-init", atomic: true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			item, directory, pending, policy := copiedRecoveryInitializationFixture(t, sample.empty, sample.atomic)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			release, results, stop := copiedRecoveryInitializationWorker(ctx, pending)
			defer stop()
			done := make(chan error, 1)
			go func() {
				done <- publishVariants(ctx, item.Path, directory, policy, []PlaybackQuality{{Label: "1080p", Width: 1920, Height: 1080, Bitrate: 6_128_000}}, results, 1, false)
			}()
			invalid := !sample.empty && !sample.atomic
			if !invalid {
				close(release)
			}
			select {
			case err := <-done:
				copiedRecoveryInitializationMaster(t, directory, err, invalid)
			case <-ctx.Done():
				t.Fatal("controlled publication did not settle")
			}
			if invalid {
				close(release)
			}
		})
	}
}
