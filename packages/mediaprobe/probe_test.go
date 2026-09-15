package mediaprobe

import (
	"math"
	"testing"
)

func TestParseProbeSummarizesMediaAndAudioTracks(t *testing.T) {
	t.Parallel()

	result := Parse([]byte(`{"streams":[{"codec_type":"video","codec_name":"hevc","width":3840,"height":2160},{"codec_type":"audio","codec_name":"eac3","tags":{"language":"eng"}},{"codec_type":"audio","codec_name":"aac","tags":{"title":"Director commentary"}}],"format":{"duration":"3661.2"}}`))
	if result.Summary != "3840x2160 HEVC · 1h1m1s" || len(result.Audio) != 2 || result.Audio[0].Label != "ENG · EAC3" || result.Audio[1].Label != "Director commentary · AAC" {
		t.Fatalf("probe = %#v", result)
	}
}

func TestParseProbeNormalizesPlaybackAndAccessibilityFacts(t *testing.T) { //nolint:cyclop // One fixture verifies the related playback facts together.
	t.Parallel()

	result := Parse([]byte(`{
		"streams":[
			{"index":0,"codec_type":"video","codec_name":"hevc","profile":"Main 10","level":153,"pix_fmt":"yuv420p10le","width":3840,"height":2160,"r_frame_rate":"24000/1001","sample_aspect_ratio":"1:1","color_range":"tv","color_space":"bt2020nc","color_transfer":"smpte2084","color_primaries":"bt2020","bits_per_raw_sample":"10","side_data_list":[{"side_data_type":"Mastering display metadata"},{"side_data_type":"Content light level metadata"}]},
			{"index":2,"codec_type":"audio","codec_name":"eac3","profile":"E-AC-3+Atmos","sample_rate":"48000","channels":8,"channel_layout":"7.1","disposition":{"default":1,"visual_impaired":1},"tags":{"language":"eng","title":"English audio description"}},
			{"index":4,"codec_type":"subtitle","codec_name":"subrip","disposition":{"hearing_impaired":1},"tags":{"language":"en","title":"English SDH"}},
			{"index":5,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","disposition":{"forced":1},"tags":{"language":"jpn"}}
		],
		"format":{"format_name":"matroska,webm","duration":"3600.5","bit_rate":"18000000"}
	}`))
	if result.Container != "matroska" || result.Bitrate != 18_000_000 || result.Video.Profile != "Main 10" || result.Video.BitDepth != 10 || result.Video.HDR != "hdr10" || result.Video.SampleAspectRatio != "1:1" || result.Video.Primaries != "bt2020" || result.Video.Transfer != "smpte2084" {
		t.Fatalf("format/video facts = %#v", result)
	}
	if len(result.AudioFacts) != 1 || result.AudioFacts[0].SourceIndex != 2 || result.AudioFacts[0].Role != "description" || result.AudioFacts[0].ChannelLayout != "7.1" {
		t.Fatalf("audio facts = %#v", result.AudioFacts)
	}
	if len(result.SubtitleFacts) != 2 || !result.SubtitleFacts[0].Text || result.SubtitleFacts[0].Role != "captions" || result.SubtitleFacts[1].Text || !result.SubtitleFacts[1].Forced {
		t.Fatalf("subtitle facts = %#v", result.SubtitleFacts)
	}
}

func TestRandomAccessIntervalsAreBoundedToEligibleMarkerEdges(t *testing.T) {
	t.Parallel()

	markers := []Marker{
		{Type: "credits", Start: 50, End: 60, Source: "visual"},
		{Type: "intro", Start: -1, End: 1, Source: "manual"},
		{Type: "recap", Start: 10, End: 20, Source: "chapter"},
		{Type: "outro", Start: 19, End: 30, Source: "fingerprint"},
	}
	if got := randomAccessIntervals(markers, 60); got != "8.000%12.000,17.000%22.000,28.000%32.000" {
		t.Fatalf("intervals = %q", got)
	}
	if got := randomAccessIntervals([]Marker{{Type: "credits", Start: 50, End: 60, Source: "visual"}}, 60); got != "" {
		t.Fatalf("unreviewed credits intervals = %q", got)
	}

	oversized := make([]Marker, 33)
	for index := range oversized {
		oversized[index] = Marker{Type: "intro", Start: float64(index*2 + 1), End: float64(index*2 + 2), Source: "manual"}
	}
	if got := randomAccessIntervals(oversized, 100); got != "" {
		t.Fatalf("oversized intervals = %q", got)
	}
	for _, duration := range []float64{0, -1, math.NaN()} {
		if got := randomAccessIntervals(markers, duration); got != "" {
			t.Fatalf("duration %v intervals = %q", duration, got)
		}
	}
}

func TestVideoFactsPreserveInterlaceRotationAndDolbyBaseLayer(t *testing.T) { //nolint:cyclop // One probe response must preserve the complete video presentation metadata.
	facts := videoFactsFor(probeStream{CodecName: "hevc", Width: 1920, Height: 1080, FieldOrder: "tt", ColorTransfer: "smpte2084", PixelFormat: "yuv420p10le", SideData: []probeSideData{{Type: "Display Matrix", Rotation: -90}, {Type: "DOVI configuration record", DVProfile: 8, DVCompatibility: 1}}})
	if facts.FieldOrder != "tt" || facts.Rotation != -90 || facts.DolbyVisionProfile != 8 || facts.DolbyVisionCompatibility != 1 || facts.HDR != "dolby-vision" || facts.BitDepth != 10 {
		t.Fatalf("video facts = %#v", facts)
	}
	if !validVideoBounds(facts) {
		t.Fatal("valid video facts were rejected")
	}
	for _, rotation := range []int{-361, 45, 361} {
		invalid := facts
		invalid.Rotation = rotation
		if validVideoBounds(invalid) {
			t.Fatalf("invalid rotation %d accepted", rotation)
		}
	}
	invalid := facts
	invalid.DolbyVisionCompatibility = 16
	if validVideoBounds(invalid) {
		t.Fatal("invalid Dolby base-layer signal accepted")
	}
}
