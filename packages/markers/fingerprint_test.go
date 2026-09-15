package markers

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestFingerprintCacheAndToolFailuresAreDeterministic(t *testing.T) { //nolint:cyclop,funlen,gocognit // Cache, tool output, extraction, and filesystem failures are one analysis boundary.
	t.Parallel()
	directory := t.TempDir()
	media := filepath.Join(directory, "media.mp4")
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(directory, "fingerprint-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf '{\"fingerprint\":[1,2,3]}'\n"), 0o700); err != nil { //nolint:gosec // Test-owned executable fixture.
		t.Fatal(err)
	}
	item := library.Item{ID: "item", Path: media}
	analyzer := &Analyzer{cache: filepath.Join(directory, "cache"), tool: tool}
	points, err := analyzer.fingerprint(t.Context(), item, "head", 0, 10)
	if err != nil || !reflect.DeepEqual(points, []uint32{1, 2, 3}) {
		t.Fatalf("fingerprint = %#v, %v", points, err)
	}
	analyzer.tool = filepath.Join(directory, "missing")
	if cached, cacheErr := analyzer.fingerprint(t.Context(), item, "head", 0, 10); cacheErr != nil || !reflect.DeepEqual(cached, points) {
		t.Fatalf("cached fingerprint = %#v, %v", cached, cacheErr)
	}
	if _, err = (&Analyzer{}).fingerprint(t.Context(), item, "head", 0, 10); err == nil {
		t.Fatal("missing cache configuration accepted")
	}
	badTool := filepath.Join(directory, "bad-tool")
	if err = os.WriteFile(badTool, []byte("#!/bin/sh\nprintf '{}'\n"), 0o700); err != nil { //nolint:gosec // Test-owned executable fixture.
		t.Fatal(err)
	}
	analyzer.tool = badTool
	if _, err = analyzer.fingerprint(t.Context(), item, "tail", 0, 10); err == nil {
		t.Fatal("empty fingerprint accepted")
	}
	failing, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false executable is unavailable")
	}
	analyzer.tool = failing
	if _, err = analyzer.fingerprint(t.Context(), item, "other", 0, 10); err == nil {
		t.Fatal("fingerprint tool failure accepted")
	}
	analyzer.ffmpeg, analyzer.tool = failing, tool
	if _, err = analyzer.fingerprint(t.Context(), item, "offset", 1, 10); err == nil {
		t.Fatal("audio extraction failure accepted")
	}
	blocked := filepath.Join(directory, "blocked")
	if err = os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	analyzer.cache = blocked
	if _, err = analyzer.fingerprint(t.Context(), item, "blocked", 1, 10); err == nil {
		t.Fatal("invalid cache directory accepted")
	}
}
