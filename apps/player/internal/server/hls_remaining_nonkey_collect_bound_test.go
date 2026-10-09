package server

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/servertest"
)

// A public ready response cannot prove serial-probe ownership or detect a
// same-byte source-inode swap. Real process/file stand-ins cover those gaps.
func TestRemainingNonKeySourceClockOperationBounds(t *testing.T) {
	for _, name := range []string{"shared-two-process-deadline", "inherited-stdout", "same-byte-source-replacement"} {
		t.Run(name, func(t *testing.T) {
			remainingNonKeyCollectorOperationBound(t, name)
		})
	}
}

func remainingNonKeyCollectorOperationBound(t *testing.T, name string) {
	t.Helper()
	manager, item, _, _ := hlsLoadingFixture(t)
	manager.index = &libraryIndex{Index: catalog.NewMemoryIndex(nil, true)}
	manager.index.SetRoots([]catalog.ScanRoot{{Path: filepath.Dir(item.Path)}})
	recipe := hlsRecipe{mode: "remux", offset: 12.5}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal("source-clock controlled policy")
	}
	fixture := remainingNonKeySourceAudio(t, 1000, 1024)
	var native bytes.Buffer
	if json.Indent(&native, fixture.encode(t), "", "  ") != nil {
		t.Fatal("source-clock controlled JSON")
	}
	tools := t.TempDir()
	marker := filepath.Join(tools, "owned-pids")
	nativePath, normalizedPath := filepath.Join(tools, "native.json"), filepath.Join(tools, "normalized.txt")
	writeHLSLoadingFile(t, nativePath, native.String())
	writeHLSLoadingFile(t, normalizedPath, fixture.normalized)
	probeAction, normalizedAction := remainingNonKeyCollectorActions(name, item.Path, marker, nativePath)
	manager.probe.executable = remainingNonKeyCollectorAdapter(t, tools, "probe", marker, nativePath, probeAction)
	manager.ffmpeg = remainingNonKeyCollectorAdapter(t, tools, "normalize", marker, normalizedPath, normalizedAction)
	before := remainingNonKeyCollectorHash(t, item.Path)
	original, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal("source-clock original source identity")
	}
	started := time.Now()
	proof, err := manager.measureCopiedHLSSourceAudio(t.Context(), item, recipe, options.Cache, fixture.first, 12_500_000, fixture.edit)
	elapsed := time.Since(started)
	if err == nil || proof != nil {
		t.Fatal("nonkey invalid actual source-clock operation acquired proof")
	}
	if elapsed > 2*time.Second {
		t.Fatal("nonkey source-clock operation exceeded original shared two-second budget")
	}
	remainingNonKeyCollectorBoundWitness(t, manager, item.Path, marker, name, original, before, elapsed)
}

func remainingNonKeyCollectorActions(name, sourcePath, marker, nativePath string) (string, string) {
	probeAction, normalizedAction := "", ""
	switch name {
	case "shared-two-process-deadline":
		probeAction, normalizedAction = "sleep 1.1\n", "sleep 1.1\n"
	case "inherited-stdout":
		probeAction = "(sleep 3; cat " + copiedRecoveryQuote(nativePath) + ") &\nprintf '%s\\n' $! >> " + copiedRecoveryQuote(marker) + "\nexit 0\n"
	case "same-byte-source-replacement":
		source := copiedRecoveryQuote(sourcePath)
		replacement := copiedRecoveryQuote(sourcePath + ".replacement")
		normalizedAction = "cp " + source + " " + replacement + "\ntouch -r " + source + " " + replacement + "\nmv " + replacement + " " + source + "\n"
	}
	return probeAction, normalizedAction
}

func remainingNonKeyCollectorBoundWitness(t *testing.T, manager *hlsManager, source, marker, name string, original os.FileInfo, before [32]byte, elapsed time.Duration) {
	t.Helper()
	data, err := os.ReadFile(marker)
	if err != nil || len(data) == 0 || len(data) > 1024 {
		t.Fatal("nonkey source-clock controlled processes never executed")
	}
	pids := strings.Fields(string(data))
	if len(pids) != 2 {
		t.Fatal("nonkey source-clock expected two actual owned process markers")
	}
	for _, pid := range pids {
		copiedRecoveryAssertStopped(t, []byte(pid))
	}
	if remainingNonKeyCollectorHash(t, source) != before {
		t.Fatal("nonkey source-clock rejection changed source bytes")
	}
	remainingNonKeyCollectorReplacement(t, source, name, original)
	entries, err := os.ReadDir(manager.cache)
	if err != nil || len(entries) != 0 {
		t.Fatal("nonkey source-clock rejection exposed cache output")
	}
	t.Logf("nonkey actual-clock-rejection case=%s elapsed_ns=%d markers=%d settled=true cache_files=0 source_unchanged=true",
		name, elapsed.Nanoseconds(), len(pids))
}

func remainingNonKeyCollectorReplacement(t *testing.T, source, name string, original os.FileInfo) {
	t.Helper()
	if name == "same-byte-source-replacement" {
		replaced, err := os.Stat(source)
		if err != nil || os.SameFile(original, replaced) || original.Size() != replaced.Size() || !original.ModTime().Equal(replaced.ModTime()) {
			t.Fatal("nonkey source-clock controlled replacement did not retain bytes and timestamps on a different inode")
		}
	}
}

func remainingNonKeyCollectorAdapter(t *testing.T, tools, name, marker, output, action string) string {
	t.Helper()
	executable := filepath.Join(tools, name)
	body := "#!/bin/sh\nset -eu\nprintf '%s\\n' $$ >> " + copiedRecoveryQuote(marker) + "\n" + action +
		"exec cat " + copiedRecoveryQuote(output) + "\n"
	servertest.WriteExecutable(t, executable, body)
	return executable
}

func TestRemainingNonKeySourceClockRejectsRevokedRoot(t *testing.T) {
	manager, item, _, _ := hlsLoadingFixture(t)
	manager.index = &libraryIndex{Index: catalog.NewMemoryIndex(nil, true)}
	manager.index.SetRoots([]catalog.ScanRoot{{Path: filepath.Dir(item.Path)}})
	recipe := hlsRecipe{mode: "remux", offset: 12.5}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal("source-clock revoked-root policy")
	}
	fixture := remainingNonKeySourceAudio(t, 1000, 1024)
	var native bytes.Buffer
	if json.Indent(&native, fixture.encode(t), "", "  ") != nil {
		t.Fatal("source-clock revoked-root JSON")
	}
	tools := t.TempDir()
	marker, nativePath := filepath.Join(tools, "owned-pids"), filepath.Join(tools, "native.json")
	writeHLSLoadingFile(t, nativePath, native.String())
	manager.probe.executable = remainingNonKeyCollectorAdapter(t, tools, "probe", marker, nativePath, "sleep 0.2\n")
	normalizedPath := filepath.Join(tools, "normalized.txt")
	writeHLSLoadingFile(t, normalizedPath, fixture.normalized)
	manager.ffmpeg = remainingNonKeyCollectorAdapter(t, tools, "normalize", marker, normalizedPath, "")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	revoked := remainingNonKeyCollectorRevoke(ctx, manager, marker)
	proof, err := manager.measureCopiedHLSSourceAudio(ctx, item, recipe, options.Cache, fixture.first, 12_500_000, fixture.edit)
	cancel()
	if err == nil || proof != nil || !<-revoked {
		t.Fatal("nonkey source-clock did not reject an actually revoked source root")
	}
	data, err := os.ReadFile(marker)
	if err != nil || len(strings.Fields(string(data))) != 1 {
		t.Fatal("nonkey revoked root launched another source process")
	}
	copiedRecoveryAssertStopped(t, []byte(strings.TrimSpace(string(data))))
}

func remainingNonKeyCollectorRevoke(ctx context.Context, manager *hlsManager, marker string) <-chan bool {
	revoked := make(chan bool, 1)
	go func() {
		for ctx.Err() == nil {
			if data, err := os.ReadFile(marker); err == nil && len(data) > 0 {
				manager.index.SetRoots(nil)
				revoked <- true
				return
			}
			time.Sleep(time.Millisecond)
		}
		revoked <- false
	}()
	return revoked
}
