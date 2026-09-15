package playback

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func TestHLSPlaylistLifecycle(t *testing.T) { //nolint:cyclop // One test covers the complete publication lifecycle.
	root := t.TempDir()
	source := filepath.Join(root, "movie.mkv")
	directory := filepath.Join(root, "0123456789abcdef", "360p")
	writeTestFile(t, source, "source")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "init.mp4"), string(mp4fixture.Initialization(640, 360, "h264", "aac", "")))
	writeTestFile(t, filepath.Join(directory, "segment-00000.m4s"), strings.Repeat("x", 1000))
	manifest := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2.000,\nsegment-00000.m4s\n#EXT-X-ENDLIST\n"
	writeTestFile(t, filepath.Join(directory, "index.m3u8"), manifest)
	future := time.Now().Add(time.Second)
	for _, path := range []string{filepath.Join(directory, "index.m3u8"), filepath.Join(directory, "init.mp4"), filepath.Join(directory, "segment-00000.m4s")} {
		if err := os.Chtimes(path, future, future); err != nil {
			t.Fatal(err)
		}
	}
	quality := PlaybackQuality{Label: "360p", Width: 640, Height: 360, Bitrate: 493_000, FrameRate: 23.976}
	if !VariantReady(source, directory) || !VariantsReady(source, filepath.Dir(directory), []PlaybackQuality{quality}) || !FinalizedVariant(directory) {
		t.Fatal("complete variant was not ready")
	}
	average, peak := VariantBandwidth(directory, 1)
	if average != 4000 || peak != 4000 {
		t.Fatalf("bandwidth = %d, %d", average, peak)
	}
	write := func(path string, data []byte) error { return os.WriteFile(path, data, 0o600) }
	master := filepath.Join(filepath.Dir(directory), "index.m3u8")
	if err := WriteMaster(master, "ffmpeg-v1", "avc1.64002a,mp4a.40.2", []PlaybackQuality{quality}, true, write); err != nil {
		t.Fatal(err)
	}
	if !MasterFresh(master, source, "ffmpeg-v1") || !CacheFresh(master, source, "ffmpeg-v1") {
		t.Fatal("final cache was not fresh")
	}
	writeTestFile(t, filepath.Join(filepath.Dir(directory), ".seekable"), "ffmpeg-v1")
	if !SeekCacheFresh(filepath.Dir(directory), source, "ffmpeg-v1") {
		t.Fatal("seek cache was not fresh")
	}
	if err := FinalizePlaylist(filepath.Join(directory, "index.m3u8")); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadHLSPlaylist(master); err != nil || !strings.Contains(string(data), "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("master = %q, %v", data, err)
	}
}

func TestHLSPlaylistRejectsIncompleteAndUnsafeInputs(t *testing.T) { //nolint:cyclop // Negative playlist cases prove the shared trust boundary.
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeTestFile(t, source, "source")
	directory := filepath.Join(root, "0123456789abcdef", "360p")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n")
	if VariantReady(source, directory) || FinalizedVariant(directory) || FinalizePlaylist(filepath.Join(directory, "index.m3u8")) == nil || VariantsReady(source, root, nil) {
		t.Fatal("incomplete playlist was accepted")
	}
	if average, peak := VariantBandwidth(filepath.Join(root, "missing"), 100); average != 100 || peak != 110 {
		t.Fatalf("fallback bandwidth = %d, %d", average, peak)
	}
	write := func(string, []byte) error { t.Fatal("invalid master caused a write"); return nil }
	for _, qualities := range [][]PlaybackQuality{nil, {{Label: "../bad", Width: 1, Height: 1, Bitrate: 1}}, {{Label: "360p", Width: 0, Height: 360, Bitrate: 1}}} {
		if err := WriteMaster(filepath.Join(root, "master.m3u8"), "ffmpeg", "avc1", qualities, false, write); err == nil {
			t.Fatalf("accepted %#v", qualities)
		}
	}
	if err := WriteMaster(filepath.Join(root, "master.m3u8"), "bad\nvalue", "avc1", []PlaybackQuality{{Label: "360p", Width: 640, Height: 360, Bitrate: 1}}, false, write); err == nil {
		t.Fatal("accepted transcoder header injection")
	}
	oversized := filepath.Join(root, "huge.m3u8")
	writeTestFile(t, oversized, strings.Repeat("x", maximumHLSPlaylistBytes+1))
	if _, err := ReadHLSPlaylist(oversized); err == nil {
		t.Fatal("accepted oversized playlist")
	}
	if err := os.Symlink(filepath.Join(directory, "index.m3u8"), filepath.Join(root, "link.m3u8")); err == nil {
		if _, err := ReadHLSPlaylist(filepath.Join(root, "link.m3u8")); err == nil {
			t.Fatal("accepted symlink playlist")
		}
	}
}

func TestPublishVariantsCoversSuccessAndFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeTestFile(t, source, "source")
	directory := filepath.Join(root, "cache")
	quality := PlaybackQuality{Label: "360p", Width: 640, Height: 360, Bitrate: 493_000}
	results := make(chan error, 1)
	results <- nil
	if err := PublishVariants(context.Background(), source, directory, "ffmpeg", "avc1", []PlaybackQuality{quality}, results, 1, false, osWriteFile); err == nil {
		t.Fatal("published without a rendition")
	}
	results = make(chan error, 1)
	results <- errors.New("encode failed")
	if err := PublishVariants(context.Background(), source, directory, "ffmpeg", "avc1", []PlaybackQuality{quality}, results, 1, false, osWriteFile); err == nil || err.Error() != "encode failed" {
		t.Fatalf("encode error = %v", err)
	}
	closed := make(chan error)
	close(closed)
	if err := PublishVariants(context.Background(), source, directory, "ffmpeg", "avc1", []PlaybackQuality{quality}, closed, 1, false, osWriteFile); err == nil {
		t.Fatal("accepted an early closed result channel")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := PublishVariants(ctx, source, directory, "ffmpeg", "avc1", []PlaybackQuality{quality}, make(chan error), 1, false, osWriteFile); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	if err := PublishVariants(context.Background(), source, directory, "ffmpeg", "avc1", []PlaybackQuality{quality}, make(chan error), 0, false, osWriteFile); err == nil {
		t.Fatal("accepted invalid worker count")
	}
}

func TestHLSCacheOperationsIgnoreUnownedData(t *testing.T) { //nolint:cyclop // Cache ownership and failure cases share one no-side-effect test.
	root := t.TempDir()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	old := filepath.Join(root, "0123456789abcdef")
	active := filepath.Join(root, "fedcba9876543210-audio-1")
	unowned := filepath.Join(root, "notes")
	for _, directory := range []string{old, active, unowned} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(directory, "data"), strings.Repeat("x", 10))
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if size, err := HLSCacheStats(root, policy); err != nil || size != 20 {
		t.Fatalf("cache stats = %d, %v", size, err)
	}
	remaining, err := PruneHLSCache(root, 10, func(name string) bool { return name == filepath.Base(active) }, policy)
	if err != nil || remaining != 10 || pathExists(old) || !pathExists(active) || !pathExists(unowned) {
		t.Fatalf("prune = %d, %v", remaining, err)
	}
	if _, err := PruneHLSCache(root, -1, func(string) bool { return false }, policy); err == nil {
		t.Fatal("accepted a negative cache limit")
	}
	if err := ClearHLSCache(root, policy); err != nil || pathExists(active) || !pathExists(unowned) {
		t.Fatalf("clear = %v", err)
	}
	if size, err := HLSCacheStats("", policy); err != nil || size != 0 {
		t.Fatalf("empty cache stats = %d, %v", size, err)
	}
}

func TestHLSCacheNamesAndFilesAreBounded(t *testing.T) {
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	for name, valid := range map[string]bool{
		"0123456789abcdef": true, "0123456789abcdef-audio-1": true, "0123456789abcdef-audio-32": false,
		"not-hex-identifier": false, "not-valid-plan-t-a0-s0-none-t0-b0": false, "0123456789abcdef-plan-t-a0-s0-none-t0-b0": true,
	} {
		if HLSCacheDirectory(name, policy) != valid {
			t.Errorf("cache name %q validity changed", name)
		}
	}
	for name, valid := range map[string]bool{"index.m3u8": true, "360p/init.mp4": true, "360p/segment-00000.m4s": true, "../secret": false, "360p/segment-x0000.m4s": false} {
		if HLSFile(name) != valid {
			t.Errorf("HLS file %q validity changed", name)
		}
	}
	for value, valid := range map[string]bool{"": true, "0": true, "31": true, "-1": false, "32": false, "x": false} {
		_, err := AudioTrackIndex(value)
		if (err == nil) != valid {
			t.Errorf("track %q validity changed: %v", value, err)
		}
	}
	if HLSCacheKey("id", 0) != "id" || HLSCacheKey("id", 2) != "id-audio-2" || SourceVersion("/missing") != "missing" {
		t.Fatal("cache identifiers changed")
	}
}

func TestHLSDiagnosticsBoundOutputAndPreserveCause(t *testing.T) { //nolint:cyclop // Redaction, bounds, and subprocess behavior form one diagnostic contract.
	t.Parallel()
	cause := errors.New("exit status 1")
	failure := NewHLSDiagnosticError(cause, strings.Repeat("x", HLSDiagnosticLimit)+" /private/Film.mkv", "/private/Film.mkv")
	if !errors.Is(failure, cause) || failure.Error() != "compatible playback failed" || strings.Contains(failure.Detail(), "Film") || len(failure.Detail()) > HLSDiagnosticLimit {
		t.Fatalf("diagnostic = %q", failure.Detail())
	}
	if HLSDiagnostic(failure) != failure.Detail() || HLSDiagnostic(nil) != "" {
		t.Fatal("diagnostic projection changed")
	}
	var buffer HLSDiagnosticBuffer
	data := []byte(strings.Repeat("y", HLSDiagnosticLimit+10))
	if written, err := buffer.Write(data); err != nil || written != len(data) || len(buffer.Bytes()) != HLSDiagnosticLimit {
		t.Fatalf("buffer = %d, %v, %d", written, err, len(buffer.Bytes()))
	}
	if err := RunHLSCommand(nil); err == nil {
		t.Fatal("accepted a missing command")
	}
	if err := RunHLSCommand(exec.CommandContext(t.Context(), "sh", "-c", "printf private >&2; exit 1"), "private"); err == nil || HLSDiagnostic(err) != "<private>" {
		t.Fatalf("command diagnostic = %q", HLSDiagnostic(err))
	}
	if err := RunHLSCommand(exec.CommandContext(t.Context(), "sh", "-c", "exit 0")); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func osWriteFile(path string, data []byte) error { return os.WriteFile(path, data, 0o600) }

func pathExists(path string) bool { _, err := os.Stat(path); return err == nil }
