package playback

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"

	"github.com/MikeO7/kinosail/packages/library"
)

type failingCacheEntry struct {
	name    string
	infoErr error
}

func (entry failingCacheEntry) Name() string               { return entry.name }
func (failingCacheEntry) IsDir() bool                      { return true }
func (failingCacheEntry) Type() fs.FileMode                { return fs.ModeDir }
func (entry failingCacheEntry) Info() (fs.FileInfo, error) { return nil, entry.infoErr }

func TestHLSCacheErrorAndEvictionEdges(t *testing.T) { //nolint:cyclop,funlen,gocognit // One boundary matrix covers independent cache failures and eviction behavior.
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	missing := filepath.Join(t.TempDir(), "missing")
	if size, err := HLSCacheStats(missing, policy); err != nil || size != 0 {
		t.Fatalf("missing stats = %d, %v", size, err)
	}
	if err := ClearHLSCache("", policy); err != nil {
		t.Fatal(err)
	}
	if err := ClearHLSCache(missing, policy); err != nil {
		t.Fatal(err)
	}
	if remaining, err := PruneHLSCache("", 0, func(string) bool { return false }, policy); err != nil || remaining != 0 {
		t.Fatalf("empty prune = %d, %v", remaining, err)
	}
	if remaining, err := PruneHLSCache(missing, 0, func(string) bool { return false }, policy); err != nil || remaining != 0 {
		t.Fatalf("missing prune = %d, %v", remaining, err)
	}
	if _, err := PruneHLSCache(t.TempDir(), 0, nil, policy); err == nil {
		t.Fatal("accepted nil active callback")
	}
	file := filepath.Join(t.TempDir(), "file")
	writeTestFile(t, file, "x")
	for name, operation := range map[string]func() error{
		"stats": func() error { _, err := HLSCacheStats(file, policy); return err },
		"clear": func() error { return ClearHLSCache(file, policy) },
		"prune": func() error { _, err := PruneHLSCache(file, 0, func(string) bool { return false }, policy); return err },
	} {
		if err := operation(); err == nil {
			t.Errorf("%s accepted a file cache root", name)
		}
	}
	valid := "0123456789abcdef"
	if _, _, err := prunableHLSEntries(missing, []os.DirEntry{failingCacheEntry{name: valid}}, func(string) bool { return false }, policy); err == nil {
		t.Fatal("ignored cache walk failure")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, valid), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prunableHLSEntries(root, []os.DirEntry{failingCacheEntry{name: valid, infoErr: errors.New("info")}}, func(string) bool { return false }, policy); err == nil {
		t.Fatal("ignored cache info failure")
	}
	candidates := []hlsCacheEntry{{name: "fedcba9876543210", size: 1, modified: time.Now()}, {name: valid, size: 1, modified: time.Now().Add(-time.Hour)}}
	if remaining, err := evictHLSCacheEntries(root, candidates, 1, 2); err != nil || remaining != 1 {
		t.Fatalf("bounded eviction = %d, %v", remaining, err)
	}
	if _, err := evictHLSCacheEntries(file, candidates[:1], 1, 0); err == nil {
		t.Fatal("ignored cache removal failure")
	}
	pruneRoot := t.TempDir()
	for index, name := range []string{"0123456789abcdef", "fedcba9876543210"} {
		directory := filepath.Join(pruneRoot, name)
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(directory, "segment.ts"), "x")
		modified := time.Now().Add(time.Duration(index) * time.Hour)
		if err := os.Chtimes(directory, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	if remaining, err := PruneHLSCache(pruneRoot, 0, func(string) bool { return false }, policy); err != nil || remaining != 0 {
		t.Fatalf("ordered prune = %d, %v", remaining, err)
	}
}

func TestHLSCacheControlSerializesOperations(t *testing.T) {
	root := t.TempDir()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	var lock sync.Mutex
	busy := true
	control := NewHLSCacheControl(root, &lock, func(string) bool { return false }, func() bool { return busy }, policy)
	if size, err := control.Stats(); err != nil || size != 0 {
		t.Fatalf("stats = %d, %v", size, err)
	}
	if err := control.Clear(); err == nil {
		t.Fatal("busy cache was cleared")
	}
	busy = false
	if err := control.Clear(); err != nil {
		t.Fatal(err)
	}
	if remaining, err := control.Prune(0); err != nil || remaining != 0 {
		t.Fatalf("prune = %d, %v", remaining, err)
	}
}

func TestPlaybackDecisionRemainingBranches(t *testing.T) {
	index := 4
	plan := PlaybackPlan{SubtitleMode: "none"}
	applySubtitle(&plan, []SubtitleFacts{{Index: index, Codec: "vtt", Text: true}}, &index, BrowserCapabilities())
	if plan.SubtitleMode != "embedded" {
		t.Fatalf("embedded subtitle mode = %q", plan.SubtitleMode)
	}
	compatible := compatibility{container: false, video: true, audio: true, size: true, bitrate: true, hdr: true}
	choosePlaybackMode(&plan, NetworkIntent{}, BrowserCapabilities(), compatible)
	if plan.Mode != "remux" || plan.Reason != "container-unsupported" {
		t.Fatalf("container fallback = %#v", plan)
	}
}

func TestHLSPlaylistPublicationAndFreshnessEdges(t *testing.T) { //nolint:cyclop,funlen,gocognit // One boundary matrix covers independent publication and freshness failures.
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeTestFile(t, source, "source")
	directory := filepath.Join(root, "cache")
	quality := PlaybackQuality{Label: "360p", Width: 640, Height: 360, Bitrate: 493_000}
	makeReadyVariant(t, filepath.Join(directory, quality.Label))
	result := make(chan error, 1)
	result <- nil
	writeErr := errors.New("write")
	if err := PublishVariants(t.Context(), source, directory, "ffmpeg", "avc1", []PlaybackQuality{quality}, result, 1, false, func(string, []byte) error { return writeErr }); !errors.Is(err, writeErr) {
		t.Fatalf("early publication error = %v", err)
	}
	result = make(chan error, 1)
	result <- nil
	writes := 0
	if err := PublishVariants(t.Context(), source, directory, "ffmpeg", "avc1", []PlaybackQuality{quality}, result, 1, false, func(path string, data []byte) error {
		writes++
		return os.WriteFile(path, data, 0o600)
	}); err != nil || writes != 2 {
		t.Fatalf("publication = writes %d, %v", writes, err)
	}
	emptyVariant := filepath.Join(root, "empty")
	if err := os.MkdirAll(emptyVariant, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(emptyVariant, "index.m3u8"), "#EXTM3U\n")
	if average, peak := VariantBandwidth(emptyVariant, 100); average != 100 || peak != 110 {
		t.Fatalf("empty bandwidth = %d, %d", average, peak)
	}
	missingMaster := filepath.Join(root, "missing.m3u8")
	if MasterFresh(missingMaster, source, "ffmpeg") || CacheFresh(missingMaster, source, "ffmpeg") || SeekCacheFresh(root, source, "ffmpeg") {
		t.Fatal("missing caches reported fresh")
	}
	for name, manifest := range map[string]string{
		"missing transcoder": "#EXTM3U\n#EXT-X-STREAM-INF:\n",
		"missing stream":     "#EXTM3U\n#KINOSAIL-TRANSCODER:ffmpeg\n",
	} {
		path := filepath.Join(root, strings.ReplaceAll(name, " ", "-")+".m3u8")
		writeTestFile(t, path, manifest)
		if MasterFresh(path, source, "ffmpeg") {
			t.Errorf("%s master reported fresh", name)
		}
	}
	invalidMaster := filepath.Join(root, "invalid-master.m3u8")
	writeTestFile(t, invalidMaster, "#EXTM3U\n#KINOSAIL-TRANSCODER:ffmpeg\n#EXT-X-STREAM-INF:\nbad/index.m3u8\n")
	if CacheFresh(invalidMaster, source, "ffmpeg") {
		t.Fatal("invalid variant reported fresh")
	}
	invalidVariant := filepath.Join(root, "invalid-variant")
	if err := os.MkdirAll(invalidVariant, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(invalidVariant, "index.m3u8"), "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"bad.mp4\"\n#EXTINF:1,\nsegment-00000.m4s\n#EXT-X-ENDLIST\n")
	if FinalizedVariant(invalidVariant) {
		t.Fatal("invalid init file reported finalized")
	}
	missingSegment := filepath.Join(root, "missing-segment")
	if err := os.MkdirAll(missingSegment, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(missingSegment, "index.m3u8"), "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:1,\nsegment-00000.m4s\n#EXT-X-ENDLIST\n")
	writeTestFile(t, filepath.Join(missingSegment, "init.mp4"), string(mp4fixture.Initialization(640, 360, "h264", "aac", "")))
	if FinalizedVariant(missingSegment) {
		t.Fatal("missing segment reported finalized")
	}
	if version := SourceVersion(source); version == "missing" || !strings.HasPrefix(version, "6:") {
		t.Fatalf("source version = %q", version)
	}
	if err := FinalizePlaylist(filepath.Join(root, "missing-playlist")); err == nil {
		t.Fatal("finalized a missing playlist")
	}
}

func TestPlaybackRecipeProtocolAndSubtitleEdges(t *testing.T) { //nolint:cyclop // One boundary matrix covers independent recipe and subtitle constraints.
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	if recipe := RecipeFor(PlaybackPlan{Mode: "transcode", SubtitleMode: "burn-in"}); recipe.Burn != "image" {
		t.Fatalf("image recipe = %#v", recipe)
	}
	for _, token := range []string{"t-a0-s0-none-t0-b0-ox", "t-a0-s0-none-t0-x0"} {
		if _, err := ParseHLSRecipe(token, policy); err == nil {
			t.Errorf("accepted %q", token)
		}
	}
	start, window := HLSWindowRecipe(HLSRecipe{Offset: 5, Omitted: []Range{{Start: 10, End: 20}}}, 100)
	if start != 5 || len(window.Omitted) != 1 || window.Omitted[0] != (Range{Start: 5, End: 15}) {
		t.Fatalf("window = %v, %#v", start, window)
	}
	if got := automaticSkipVideoFilter(nil, false); got != "" {
		t.Fatalf("empty skip filter = %q", got)
	}
	for _, value := range []string{"00:bad", "bad:00", "bad:00:00"} {
		if _, err := ParseVTTTime(value); err == nil {
			t.Errorf("accepted VTT time %q", value)
		}
	}
	badVTT := []byte("WEBVTT\n\n00:bad --> 00:01.000\ntext")
	if got := string(MapWebVTT(badVTT, Timeline{Duration: 10, SourceDuration: 10})); !strings.Contains(got, "00:bad") {
		t.Fatalf("invalid cue was lost: %q", got)
	}
	if selected := selectAudio([]AudioFacts{{Index: 3, Codec: "aac"}}, []string{"aac"}, NetworkIntent{}, DecisionPolicy{}); selected.Index != 3 {
		t.Fatalf("fallback audio = %#v", selected)
	}
	if got := AdaptiveQualities(2, 2, 0, 0); len(got) != 1 || got[0].Width != 2 {
		t.Fatalf("tiny ladder = %#v", got)
	}
}

func TestHLSDeliveryAndJellyfinValidationEdges(t *testing.T) { //nolint:cyclop // One boundary matrix covers independent delivery and provider constraints.
	if _, err := HLSCodecArguments(HLSCodecInput{AudioRate: "128k", CopyInput: "0", Recipe: HLSRecipe{Mode: "transcode"}}); err == nil {
		t.Fatal("accepted missing video rate")
	}
	if _, err := HLSCodecArguments(HLSCodecInput{AudioRate: "128k", CopyInput: "0", Recipe: HLSRecipe{Mode: "bad"}}); err == nil {
		t.Fatal("accepted playback mode")
	}
	now := time.Now()
	session := testJellyfinSession{item: "item", expires: now.Add(time.Minute)}
	if _, err := StoreJellyfinPlaySession(nil, now, func() (string, error) { return "id", nil }, session); err == nil {
		t.Fatal("accepted nil session store")
	}
	if _, err := StoreJellyfinPlaySession(&syncMap, now, func() (string, error) { return "", nil }, session); err == nil {
		t.Fatal("accepted empty session ID")
	}
	oversizedProfiles := make([]JellyfinMediaProfile, 129)
	if validJellyfinMediaProfiles(oversizedProfiles) {
		t.Fatal("accepted too many media profiles")
	}
	if validJellyfinMediaProfiles([]JellyfinMediaProfile{{Container: strings.Repeat("x", 1025)}}) {
		t.Fatal("accepted oversized media profile")
	}
	if validJellyfinSubtitleProfiles(make([]JellyfinSubtitleProfile, 129)) {
		t.Fatal("accepted too many subtitle profiles")
	}
	profile := JellyfinDeviceProfile{DirectPlayProfiles: []JellyfinMediaProfile{{Type: "Audio", Container: "mp4"}}}
	if capabilities := JellyfinCapabilities(profile, MediaFacts{Kind: "video"}); len(capabilities.Containers) != 0 {
		t.Fatalf("mismatched capabilities = %#v", capabilities)
	}
	facts := MediaFacts{Subtitles: []SubtitleFacts{{External: true}, {SourceIndex: 2, Text: true}}}
	if streams := JellyfinMediaStreams(library.Item{ID: "id"}, facts, ""); len(streams) != 1 {
		t.Fatalf("subtitle streams = %#v", streams)
	}
	plain := errors.New("plain failure")
	if got := HLSDiagnostic(plain); got != plain.Error() {
		t.Fatalf("plain diagnostic = %q", got)
	}
	if failure := NewHLSDiagnosticError(nil, ""); failure.Unwrap() == nil || failure.Detail() == "" {
		t.Fatalf("default diagnostic = %#v", failure)
	}
}

var syncMap = sync.Map{}

func makeReadyVariant(t *testing.T, directory string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "init.mp4"), string(mp4fixture.Initialization(640, 360, "h264", "aac", "")))
	writeTestFile(t, filepath.Join(directory, "segment-00000.m4s"), "segment")
	writeTestFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:1,\nsegment-00000.m4s\n#EXT-X-ENDLIST\n")
	future := time.Now().Add(time.Second)
	for _, path := range []string{filepath.Join(directory, "index.m3u8"), filepath.Join(directory, "init.mp4"), filepath.Join(directory, "segment-00000.m4s")} {
		if err := os.Chtimes(path, future, future); err != nil {
			t.Fatal(err)
		}
	}
}
