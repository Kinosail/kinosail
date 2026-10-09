//go:build linux

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Test-first isolated gap: the public API cannot select a pending presentation
// origin. Invoke the actual canonical worker with a private, unproved descriptor.
// Its complete output must equal a fresh independently qualified CLI reference:
// real IDR payload, signed clocks, every video/AAC packet through physical EOF,
// source identity, worker exit and released workloads. No cache/browser admission.
func TestRemainingNonKeyActualPendingProducer(t *testing.T) {
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The bounded hosted job owns the fresh real-media reference")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("pending producer pinned FFmpeg missing")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("pending producer pinned FFprobe missing")
	}
	var reference remainingNonKeyProducerReference
	data := remainingNonKeyCollectorRead(t, os.Getenv("KINOSAIL_NONKEY_CLI_REFERENCE"), 4<<20)
	if json.Unmarshal(data, &reference) != nil || !reference.Qualified || len(reference.Cases) != 2 ||
		reference.Revision != os.Getenv("GITHUB_SHA") {
		t.Fatal("pending producer qualified independent reference missing")
	}
	for _, expected := range reference.Cases {
		t.Run(expected.Container, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			t.Cleanup(cancel)
			remainingNonKeyActualPendingProducer(t, ctx, cancel, ffmpeg, ffprobe, expected)
		})
	}
}

type remainingNonKeyProducerReference struct {
	Revision  string                        `json:"revision"`
	Qualified bool                          `json:"qualifiedIndependentReference"`
	Cases     []remainingNonKeyProducerCase `json:"cases"`
}

type remainingNonKeyProducerCase struct {
	Container     string                          `json:"container"`
	Source        string                          `json:"source"`
	SourceHash    string                          `json:"sourceSHA256"`
	Requested     int64                           `json:"requestedMicros"`
	IDRPTS        string                          `json:"requiredIDRPTS"`
	IDRHash       string                          `json:"requiredIDRPayloadSHA256"`
	ExpectedVideo []remainingNonKeyProducerPacket `json:"expectedVideo"`
	ExpectedAudio []remainingNonKeyProducerPacket `json:"expectedAudio"`
}

type remainingNonKeyProducerPacket struct {
	PTS      string `json:"pts_time"`
	DTS      string `json:"dts_time"`
	Duration string `json:"duration_time"`
	Hash     string `json:"data_hash"`
}

type remainingNonKeyProducerFixture struct {
	manager   *hlsManager
	item      library.Item
	recipe    hlsRecipe
	options   transcodeSettings
	facts     probeResult
	directory string
	timeline  *copiedHLSTimeline
	marker    string
	key       float64
}

func remainingNonKeyActualPendingProducer(t *testing.T, ctx context.Context, cancel context.CancelFunc, ffmpeg, ffprobe string, expected remainingNonKeyProducerCase) {
	t.Helper()
	fixture := remainingNonKeyProducerSetup(t, ctx, cancel, ffmpeg, ffprobe, expected)
	manager := fixture.manager
	manager.ffmpeg = remainingNonKeyRootedProcess(t, ffmpeg, fixture.marker, "")
	start, window := hlsWindowRecipe(fixture.recipe, fixture.facts.Duration)
	if manager.encodeVariant(ctx, fixture.item, fixture.directory, "360p", "640", "1000k", "128k", fixture.facts.Duration,
		fixture.options, fixture.recipe, window, start, 0) != nil {
		t.Fatal("pending producer actual canonical worker")
	}
	pids := strings.Fields(string(remainingNonKeyCollectorRead(t, fixture.marker, 1024)))
	if len(pids) != 1 {
		t.Fatal("pending producer actual worker count")
	}
	copiedRecoveryAssertStopped(t, []byte(pids[0]))
	joined, cuts := remainingNonKeyProducerJoin(t, filepath.Join(fixture.directory, "360p"), len(fixture.timeline.Keys))
	video := remainingNonKeyProducerPackets(t, ctx, ffprobe, joined, "v:0")
	audio := remainingNonKeyProducerPackets(t, ctx, ffprobe, joined, "a:0")
	if video[0].Hash != "SHA256:"+expected.IDRHash {
		t.Error("pending producer first access unit differs from the independent real IDR payload")
	}
	remainingNonKeyProducerEqual(t, video, expected.ExpectedVideo, "video")
	remainingNonKeyProducerEqual(t, audio, expected.ExpectedAudio, "audio")
	observed := remainingNonKeyProducerClock(t, video[0].PTS)
	signed := fixture.key - float64(expected.Requested)/1_000_000
	if math.Abs(observed-signed) > 0.000001 {
		t.Errorf("pending producer signed presentation origin observed=%.6f expected=%.6f", observed, signed)
	}
	remainingNonKeyProducerUnpublished(t, ctx, fixture)
	t.Logf("nonkey actual-pending-producer container=%s source_key=%.6f requested_source=%.6f expected_signed=%.6f observed_signed=%.6f physical_cuts=%d video_packets=%d audio_packets=%d worker_joined=true certificate_unpublished=true",
		expected.Container, fixture.key, float64(expected.Requested)/1_000_000, signed, observed, cuts, len(video), len(audio))
}

func remainingNonKeyProducerSetup(t *testing.T, ctx context.Context, cancel context.CancelFunc, ffmpeg, ffprobe string, expected remainingNonKeyProducerCase) remainingNonKeyProducerFixture {
	t.Helper()
	manager, item, _, _ := hlsLoadingFixtureContext(t, ctx)
	item.Path = expected.Source
	manager.index = &libraryIndex{Index: catalog.NewMemoryIndex(nil, true)}
	manager.index.SetRoots([]catalog.ScanRoot{{Path: filepath.Dir(item.Path)}})
	manager.probe, manager.ffmpeg = newMediaProbe(ffprobe), ffmpeg
	marker := filepath.Join(t.TempDir(), "owned-producer-pid")
	remainingNonKeyProducerCleanup(t, cancel, manager, item, marker, expected.SourceHash)
	recipe := hlsRecipe{mode: "remux", offset: 12.5}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal("pending producer source policy")
	}
	facts := manager.probe.facts(ctx, item)
	timeline, err := manager.scanCopiedHLSPackets(ctx, item, options.Cache, facts.Duration)
	if err != nil || manager.certifyCopiedHLS(ctx, item, timeline) != nil ||
		manager.certifyCopiedHLSConfiguration(ctx, item, timeline) != nil {
		t.Fatal("pending producer actual complete IDR/configuration certification")
	}
	key := remainingNonKeyProducerClock(t, expected.IDRPTS)
	selected := -1
	for number := range timeline.Keys {
		if math.Abs(timeline.point(number)-key) <= 0.000001 {
			selected = number
		}
	}
	if selected < 0 || selected+1 >= len(timeline.Keys) || expected.Requested <= 0 {
		t.Fatal("pending producer independently measured preceding IDR")
	}
	timeline.Keys = append([]copiedHLSKey(nil), timeline.Keys[selected:]...)
	timeline.Strategy = copiedHLSPrerollStrategy
	timeline.Presentation = &copiedHLSPresentation{RequestedMicros: expected.Requested, Decode: timeline.Keys[0]}
	if !validCopiedHLSTimeline(timeline) || timeline.Presentation.Proof != nil || timeline.Clock != nil {
		t.Fatal("pending producer fixture fabricated an accepted proof")
	}
	directory := filepath.Join(manager.cache, ".copy-producer-contract")
	if os.Mkdir(directory, 0o700) != nil || playback.BindHLSSource(directory, item.Path, options.Cache) != nil ||
		manager.writeCopiedHLSTimeline(directory, timeline) != nil {
		t.Fatal("pending producer exclusive private source binding")
	}
	return remainingNonKeyProducerFixture{
		manager: manager, item: item, recipe: recipe, options: options, facts: facts,
		directory: directory, timeline: timeline, marker: marker, key: key,
	}
}

func remainingNonKeyProducerCleanup(t *testing.T, cancel context.CancelFunc, manager *hlsManager, item library.Item, marker, expectedHash string) {
	t.Helper()
	before := remainingNonKeyCollectorHash(t, item.Path)
	if fmt.Sprintf("%x", before) != expectedHash {
		t.Fatal("pending producer fixture differs from the qualified immutable source")
	}
	t.Cleanup(func() {
		if remainingNonKeyCollectorHash(t, item.Path) != before {
			t.Error("pending producer changed its immutable source")
		}
	})
	t.Cleanup(func() {
		work := manager.workloads.Metrics()
		if work.ActivePlayback != 0 || work.ActiveBackground != 0 || work.WaitingPlayback != 0 ||
			work.WaitingBackground != 0 || len(manager.jobs) != 0 {
			t.Error("pending producer retained workload or public job")
		}
	})
	t.Cleanup(func() {
		cancel()
		pids, err := os.ReadFile(marker)
		if os.IsNotExist(err) {
			return
		}
		if err != nil || len(pids) > 1024 {
			t.Error("pending producer worker witness unreadable")
			return
		}
		for _, pid := range strings.Fields(string(pids)) {
			copiedRecoveryAssertStopped(t, []byte(pid))
		}
	})
}

func remainingNonKeyProducerUnpublished(t *testing.T, ctx context.Context, fixture remainingNonKeyProducerFixture) {
	t.Helper()
	for _, absent := range []string{".copy-clock", ".startup-completion", "index.m3u8"} {
		if _, err := os.Lstat(filepath.Join(fixture.directory, absent)); !os.IsNotExist(err) {
			t.Error("pending private producer fabricated public admission")
		}
	}
	current, err := fixture.manager.readCopiedHLSTimelineContext(ctx, fixture.directory, fixture.options.Cache)
	if err != nil || current == nil || current.Clock != nil || current.Presentation == nil || current.Presentation.Proof != nil {
		t.Error("pending producer changed proof status")
	}
}

func remainingNonKeyProducerJoin(t *testing.T, media string, maximum int) (string, int) {
	t.Helper()
	manifest := remainingNonKeyCollectorRead(t, filepath.Join(media, "index.m3u8"), maximumCopiedHLSTimelineBytes)
	if !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		t.Fatal("pending producer genuine physical EOF")
	}
	joined := bytes.Clone(remainingNonKeyCollectorRead(t, filepath.Join(media, "init.mp4"), 2<<20))
	cuts := 0
	for _, line := range strings.Split(string(manifest), "\n") {
		if _, valid := hlsSegmentNumber(line); !valid {
			continue
		}
		joined = append(joined, remainingNonKeyCollectorRead(t, filepath.Join(media, line), 64<<20)...)
		cuts++
		if len(joined) > 64<<20 {
			t.Fatal("pending producer complete join bound")
		}
	}
	if cuts == 0 || cuts > maximum {
		t.Fatal("pending producer physical cut bound")
	}
	path := filepath.Join(t.TempDir(), "joined.mp4")
	if os.WriteFile(path, joined, 0o600) != nil {
		t.Fatal("pending producer private complete join")
	}
	return path, cuts
}
