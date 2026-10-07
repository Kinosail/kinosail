package server

import (
	"math"
	"strconv"
	"testing"
)

// The public origin replay proves one valid aligned refill. These guards cover
// the important gap: unrelated seeks must never acquire replay work or a new
// AAC phase. The failure inventory was written before these tests and code in
// remaining-playback-production-argument-plan-20261007.json.
type remainingAudioOriginGuard struct {
	name string
	edit func(*MediaFacts, *hlsRecipe, *hlsRecipe, *float64, *int, *string)
}

func TestRemainingAudioOriginRefillKeepsUnsupportedSeeksOrdinary(t *testing.T) {
	checkRemainingAudioOriginGuards(t, []remainingAudioOriginGuard{
		{"initial", func(_ *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, s *float64, n *int, _ *string) { *s, *n = 0, 0 }},
		{"unaligned2", func(_ *MediaFacts, a, b *hlsRecipe, s *float64, n *int, _ *string) {
			*s, *n, a.offset, a.outputTime, b.outputTime = 2, 1, 2, 2, 2
		}},
		{"unaligned4", func(_ *MediaFacts, a, b *hlsRecipe, s *float64, n *int, _ *string) {
			*s, *n, a.offset, a.outputTime, b.outputTime = 4, 2, 4, 4, 4
		}},
		{"unaligned6", func(_ *MediaFacts, a, b *hlsRecipe, s *float64, n *int, _ *string) {
			*s, *n, a.offset, a.outputTime, b.outputTime = 6, 3, 6, 6, 6
		}},
		{"laterAligned", func(f *MediaFacts, a, b *hlsRecipe, s *float64, n *int, _ *string) {
			f.Duration = 20
			*s, *n, a.offset, a.outputTime, b.outputTime = 16, 8, 16, 16, 16
		}},
		{"wrongCadence", func(_ *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, n *int, _ *string) { *n = 3 }},
		{"nan", func(_ *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, s *float64, _ *int, _ *string) { *s = math.NaN() }},
		{"infinity", func(_ *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, s *float64, _ *int, _ *string) { *s = math.Inf(1) }},
	})
}

func TestRemainingAudioOriginRefillKeepsUnsupportedSourcesOrdinary(t *testing.T) {
	checkRemainingAudioOriginGuards(t, []remainingAudioOriginGuard{
		{"wrongRate", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio[0].SampleRate = 44100
		}},
		{"mono", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio[0].Channels = 1
		}},
		{"unknownLayout", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio[0].ChannelLayout = ""
		}},
		{"otherTwoChannelLayout", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio[0].ChannelLayout = "FC+LFE"
		}},
		{"otherCodec", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio[0].Codec = "aac"
		}},
		{"otherContainer", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) { f.Container = "mkv" }},
		{"otherKind", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) { f.Kind = "video" }},
		{"unprovedAudiobook", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) { f.Kind = "audiobook" }},
		{"mixedVideo", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) { f.Video.Codec = "h264" }},
		{"missingAudio", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) { f.Audio = nil }},
		{"multipleAudio", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio = append(f.Audio, f.Audio[0])
		}},
		{"otherTrack", func(f *MediaFacts, a, b *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio[0].Index, a.audio, b.audio = 1, 1, 1
		}},
		{"otherMapping", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Audio[0].SourceIndex = 1
		}},
		{"duration", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) { f.Duration = 8 }},
		{"nanDuration", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Duration = math.NaN()
		}},
		{"infiniteDuration", func(f *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, _ *string) {
			f.Duration = math.Inf(1)
		}},
	})
}

func TestRemainingAudioOriginRefillKeepsUnsupportedRecipesOrdinary(t *testing.T) {
	checkRemainingAudioOriginGuards(t, []remainingAudioOriginGuard{
		{"nonoriginResume", func(_ *MediaFacts, _ *hlsRecipe, b *hlsRecipe, _ *float64, _ *int, _ *string) { b.outputTime = 6 }},
		{"otherBitrate", func(_ *MediaFacts, _ *hlsRecipe, _ *hlsRecipe, _ *float64, _ *int, r *string) { *r = "128000" }},
		{"boost", func(_ *MediaFacts, a, b *hlsRecipe, _ *float64, _ *int, _ *string) {
			a.dialogueBoost, b.dialogueBoost = true, true
		}},
		{"normalize", func(_ *MediaFacts, a, b *hlsRecipe, _ *float64, _ *int, _ *string) {
			a.normalizeLoudness, b.normalizeLoudness = true, true
		}},
		{"omitted", func(_ *MediaFacts, a, b *hlsRecipe, _ *float64, _ *int, _ *string) {
			a.omitted, b.omitted = []PlaybackRange{{Start: 1, End: 2}}, []PlaybackRange{{Start: 1, End: 2}}
		}},
		{"wrongMode", func(_ *MediaFacts, a, b *hlsRecipe, _ *float64, _ *int, _ *string) { a.mode, b.mode = "remux", "remux" }},
		{"sourceOffsetMismatch", func(_ *MediaFacts, a, _ *hlsRecipe, _ *float64, _ *int, _ *string) { a.offset = 10 }},
		{"sourceOutputMismatch", func(_ *MediaFacts, a, _ *hlsRecipe, _ *float64, _ *int, _ *string) { a.outputTime = 6 }},
		{"windowOffsetMismatch", func(_ *MediaFacts, _, b *hlsRecipe, _ *float64, _ *int, _ *string) { b.offset = 2 }},
		{"sourceOnlyBoost", func(_ *MediaFacts, a, _ *hlsRecipe, _ *float64, _ *int, _ *string) { a.dialogueBoost = true }},
		{"windowOnlyOmission", func(_ *MediaFacts, _, b *hlsRecipe, _ *float64, _ *int, _ *string) {
			b.omitted = []PlaybackRange{{Start: 1, End: 2}}
		}},
	})
}

func checkRemainingAudioOriginGuards(t *testing.T, samples []remainingAudioOriginGuard) {
	t.Helper()
	facts := MediaFacts{
		Kind: "audio", Container: "flac", Duration: 10,
		Audio: []AudioFacts{{Index: 0, SourceIndex: 0, Codec: "flac", SampleRate: 48000, Channels: 2, ChannelLayout: "stereo"}},
	}
	source := hlsRecipe{mode: "audio-transcode", audio: 0, subtitle: 0, offset: 8, outputTime: 8}
	window := source
	window.offset = 0
	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			f, a, b, s, n, r := facts, source, window, 8.0, 4, "192000"
			f.Audio = append([]AudioFacts(nil), facts.Audio...)
			sample.edit(&f, &a, &b, &s, &n, &r)
			if remainingAudioOriginRefill(f, a, b, s, n, r) != nil {
				t.Fatal("unsupported worker acquired source-origin replay")
			}
		})
	}
}

func TestRemainingAudioOriginRefillRetainsSamplePrecision(t *testing.T) {
	facts := MediaFacts{
		Kind: "audio", Container: "flac", Duration: 10,
		Audio: []AudioFacts{{Index: 0, SourceIndex: 0, Codec: "flac", SampleRate: 48000, Channels: 2, ChannelLayout: "stereo"}},
	}
	source := hlsRecipe{mode: "audio-transcode", audio: 0, subtitle: 0, offset: 8, outputTime: 8}
	window := source
	window.offset = 0
	result := remainingAudioOriginRefill(facts, source, window, 8, 4, "192000")
	if result == nil || result.drop != "noise=amount=0:drop=lt(pts\\,382976)" {
		t.Fatalf("missing aligned AAC packet boundary: %+v", result)
	}
	offset, err := strconv.ParseFloat(result.output, 64)
	if err != nil || math.Abs(offset*48000-1024) > 0.000001 {
		t.Fatalf("output offset lost sample precision: %q", result.output)
	}
}

// Four real AAC EXTINF values add to 7.999998999999999, while the existing
// encoder command seeks 8.000. Preserve that actual codec target.
func TestRemainingAudioOriginRefillUsesMeasuredCadenceRounding(t *testing.T) {
	facts := MediaFacts{
		Kind: "audio", Container: "flac", Duration: 10,
		Audio: []AudioFacts{{Index: 0, SourceIndex: 0, Codec: "flac", SampleRate: 48000, Channels: 2, ChannelLayout: "stereo"}},
	}
	for _, sample := range []struct {
		name     string
		start    float64
		eligible bool
	}{
		{"observedPrefix", 7.999998999999999, true},
		{"positivePrefixRounding", 8.000001, true},
		{"unqualifiedFractionBefore", 7.9999, false},
		{"unqualifiedFractionAfter", 8.0001, false},
		{"differentEncoderTarget", 8.0006, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source := hlsRecipe{mode: "audio-transcode", offset: sample.start, outputTime: sample.start}
			window := source
			window.offset = 0
			result := remainingAudioOriginRefill(facts, source, window, sample.start, 4, "192000")
			if (result != nil) != sample.eligible {
				t.Fatalf("candidate eligibility=%t; ordinary encoder target=%s", result != nil, ffmpegSeconds(sample.start))
			}
			if result != nil && result.drop != "noise=amount=0:drop=lt(pts\\,382976)" {
				t.Fatal("playlist rounding changed the actual AAC packet target")
			}
		})
	}
}

// The ten-second public control cannot cover a normalized cut at source EOF.
func TestRemainingAudioOriginRefillRejectsRoundedSourceEOF(t *testing.T) {
	facts := MediaFacts{
		Kind: "audio", Container: "flac", Duration: 8,
		Audio: []AudioFacts{{Index: 0, SourceIndex: 0, Codec: "flac", SampleRate: 48000, Channels: 2, ChannelLayout: "stereo"}},
	}
	start := 7.999998999999999
	source := hlsRecipe{mode: "audio-transcode", offset: start, outputTime: start}
	window := source
	window.offset = 0
	if remainingAudioOriginRefill(facts, source, window, start, 4, "192000") != nil {
		t.Fatal("rounded source EOF acquired a refill worker")
	}
}
