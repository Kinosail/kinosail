package mediaprobe

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProbeRemainingExecutionAndCancellationEdges(t *testing.T) { //nolint:cyclop // One white-box matrix covers independent command and coalescing edges.
	t.Parallel()
	item := library.Item{ID: "movie", Kind: "video", Path: "/missing/movie.mkv"}
	if result := New("").Inspect(t.Context(), item, Enrichment{}); !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("disabled probe = %#v", result)
	}

	probe := New("ffprobe")
	version := "missing"
	probe.calls[item.ID+"\x00"+version] = &probeCall{done: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result := probe.Inspect(ctx, item, Enrichment{}); !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("canceled coalesced probe = %#v", result)
	}
	if result := New("\x00").Inspect(t.Context(), item, Enrichment{}); !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("failed probe = %#v", result)
	}

	output := boundedOutput{remaining: 4}
	if written, err := output.Write([]byte("data")); err != nil || written != 4 || output.String() != "data" {
		t.Fatalf("bounded write = %d, %v, %q", written, err, output.String())
	}
	canceled, stop := context.WithCancel(t.Context())
	stop()
	decorated := New("").Decorate(canceled, []library.Item{{Kind: "audio"}}, Enrichment{})
	if len(decorated) != 1 {
		t.Fatalf("canceled decoration = %#v", decorated)
	}
}

func TestProbeRemainingParsingEdges(t *testing.T) { //nolint:cyclop // One parser matrix covers independent normalized metadata alternatives.
	t.Parallel()
	if result, valid := parse([]byte("{")); valid || !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("malformed probe = %#v", result)
	}
	video := videoFactsFor(probeStream{PixelFormat: "yuv420p10le", ColorTransfer: "arib-std-b67"})
	if video.BitDepth != 10 || video.HDR != "hlg" {
		t.Fatalf("HLG video = %#v", video)
	}
	video = videoFactsFor(probeStream{SideData: []probeSideData{{Type: "DOVI configuration"}}})
	if video.HDR != "dolby-vision" {
		t.Fatalf("Dolby Vision video = %#v", video)
	}
	video = videoFactsFor(probeStream{SideData: []probeSideData{{Type: "dynamic HDR plus"}}})
	if video.HDR != "hdr10+" {
		t.Fatalf("HDR10+ video = %#v", video)
	}
	subtitle := subtitleFactsFor(probeStream{Disposition: probeDisposition{Commentary: 1}}, 0)
	if subtitle.Role != "commentary" {
		t.Fatalf("commentary subtitle = %#v", subtitle)
	}
	chapters, _ := parseChapters([]struct {
		Start string            `json:"start_time"`
		End   string            `json:"end_time"`
		Tags  map[string]string `json:"tags"`
	}{{Start: "1", End: "2"}})
	if len(chapters) != 1 || chapters[0].Title != "Chapter 1" {
		t.Fatalf("untitled chapter = %#v", chapters)
	}
	if points := New("\x00").randomAccess(t.Context(), "movie", "0%1"); len(points) != 0 {
		t.Fatalf("failed random-access probe = %#v", points)
	}
}

func TestReplayGainRemainingFormats(t *testing.T) { //nolint:cyclop // Every accepted and rejected tag grammar is explicit.
	t.Parallel()
	for name, test := range map[string]struct {
		tags  map[string]string
		track float64
		album float64
		set   bool
	}{
		"standard": {map[string]string{"replaygain_track_gain": "-7.5 dB", "replaygain_album_gain": "2"}, -7.5, 2, true},
		"r128":     {map[string]string{"r128_track_gain": "256", "r128_album_gain": "-256"}, 1, -1, true},
	} {
		t.Run(name, func(t *testing.T) {
			result := ParseReplayGain(test.tags)
			if result.Track != test.track || result.Album != test.album || result.TrackSet != test.set || result.AlbumSet != test.set {
				t.Fatalf("gain = %#v", result)
			}
		})
	}
	for _, value := range []string{"1 2", "1 watts", "1 dB extra", "invalid", "NaN", "+Inf", "129", "-129"} {
		if gain, ok := gainTag(map[string]string{"gain": value}, "gain", false); ok || gain != 0 {
			t.Errorf("standard gain %q accepted", value)
		}
	}
	for _, value := range []string{"1 dB", strings.Repeat("1", 400)} {
		if gain, ok := gainTag(map[string]string{"gain": value}, "gain", true); ok || gain != 0 {
			t.Errorf("R128 gain %q accepted", value)
		}
	}
	if gain, ok := gainTag(map[string]string{"gain": "256"}, "gain", true); !ok || math.Abs(gain-1) > 0.0001 {
		t.Fatalf("R128 gain = %v, %v", gain, ok)
	}
}
