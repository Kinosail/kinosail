package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// Gap: HTTP cannot deterministically revoke a scan root after the retained probe opens.
func TestCopiedAACSourceRootRevocationCannotInstallOrReuseEligibility(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained source qualification is Linux-only")
	}
	manager, item, recipe, base, before, marker, release := copiedAACRootProbeFixture(t)
	pid := copiedAACRevokeDuringProbe(t, manager, item, recipe, marker, release)
	copiedRecoveryAssertStopped(t, pid)
	key := hlsRecipeKey(item.ID, recipe)
	if _, _, known := manager.copiedAACPolicy(key, base.Cache, before); known {
		t.Fatal("failed root proof became a completed unsupported decision")
	}
	copiedAACSourceRoots(manager, item)
	if err := manager.qualifyCopiedAAC(t.Context(), item, recipe, true); err != nil {
		t.Fatal("restored safe source qualification failed")
	}
	selected, _, known := manager.copiedAACPolicy(key, base.Cache, before)
	if !known || !selected {
		t.Fatal("restored root did not record eligibility")
	}
	manager.index.SetRoots(nil)
	if err := manager.qualifyCopiedAAC(t.Context(), item, recipe, false); err == nil {
		t.Fatal("known source decision bypassed revoked root")
	}
	after, err := os.Stat(item.Path)
	if err != nil || !sameCopiedHLSFile(before, after) {
		t.Fatal("qualification mutated its source")
	}
}

func copiedAACRevokeDuringProbe(t *testing.T, manager *hlsManager, item library.Item, recipe hlsRecipe, marker, release string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			select {
			case <-done:
				joined = true
			case <-time.After(3 * time.Second):
				t.Error("owned qualification caller did not join during cleanup")
			}
		}
	})
	go func() { done <- manager.qualifyCopiedAAC(ctx, item, recipe, true) }()
	pid := copiedAACWaitProbe(t, marker)
	manager.index.SetRoots(nil)
	writeHLSLoadingFile(t, release, "released")
	qualificationErr := <-done
	joined = true
	if qualificationErr == nil {
		t.Fatal("revoked root installed source qualification")
	}
	return pid
}

func copiedAACWaitProbe(t *testing.T, marker string) []byte {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	var pid []byte
	for time.Now().Before(deadline) {
		pid, _ = os.ReadFile(marker)
		if len(pid) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(pid) == 0 {
		t.Fatal("controlled qualification never opened its probe")
	}
	return pid
}

func copiedAACRootProbeFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe, transcodeSettings, os.FileInfo, string, string) {
	t.Helper()
	manager, item, _, _ := hlsLoadingFixture(t)
	original := item.Path
	item.Path = filepath.Join(filepath.Dir(item.Path), "Fixture.mp4")
	if err := os.Rename(original, item.Path); err != nil {
		t.Fatal(err)
	}
	copiedAACSourceRoots(manager, item)
	recipe := hlsRecipe{mode: "remux"}
	base, err := manager.baseHLSSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	marker, release := filepath.Join(tools, "opened-pid"), filepath.Join(tools, "release")
	probe := filepath.Join(tools, "grid-probe")
	body := "#!/bin/sh\nset -eu\nprintf '%s' $$ > " + copiedRecoveryQuote(marker) + "\nwhile [ ! -f " + copiedRecoveryQuote(release) + " ]; do sleep 0.01; done\nprintf '%s\\n' 'index=0|codec_type=video|codec_name=h264|profile=High|time_base=1/16000' 'index=1|codec_type=audio|codec_name=aac|profile=LC|sample_rate=48000|channels=2|time_base=1/48000' 'format_name=mov,mp4,m4a,3gp,3g2,mj2'\n"
	if err := os.WriteFile(probe, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(probe, 0o700); err != nil {
		t.Fatal(err)
	}
	manager.probe.executable = probe
	return manager, item, recipe, base, before, marker, release
}
