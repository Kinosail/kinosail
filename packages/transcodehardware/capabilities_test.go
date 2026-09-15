package transcodehardware

import (
	"reflect"
	"testing"
)

func TestCapabilityLookupAndEncoderResolution(t *testing.T) { //nolint:cyclop,gocognit // One capability snapshot exercises the mutually dependent lookup and fallback policy.
	t.Parallel()
	capabilities := Capabilities{
		Backends: []Backend{
			{ID: "none", Name: "Software", Supported: true, Usable: true, encoders: map[string]string{"h264": "libx264", "av1": "libsvtav1"}},
			{ID: "qsv", Name: "Quick Sync", Supported: true, Usable: true, encoders: map[string]string{"h264": "h264_qsv", "av1": "av1_qsv"}},
			{ID: "vaapi", Name: "VA-API", Supported: true},
		},
		Codecs:   []Codec{{ID: "h264", Name: "AVC", Supported: true, Usable: true}},
		Selected: "qsv",
		Probed:   true,
	}
	if capabilities.Resolve("auto") != "qsv" || capabilities.Resolve("none") != "none" {
		t.Fatal("backend resolution changed")
	}
	if got := capabilities.Backend("missing"); got.ID != "missing" || got.Name != "missing" {
		t.Fatalf("unknown backend = %#v", got)
	}
	if !capabilities.Supports("auto") || !capabilities.Supports("qsv") || capabilities.Supports("missing") {
		t.Fatal("backend support changed")
	}
	if got := capabilities.Codec("missing"); got.ID != "missing" || got.Name != "missing" {
		t.Fatalf("unknown codec = %#v", got)
	}
	if !capabilities.SupportsCodec("auto") || !capabilities.SupportsCodec("h264") || capabilities.SupportsCodec("missing") {
		t.Fatal("codec support changed")
	}
	tests := []struct {
		configured, codec, accelerator, encoder string
	}{
		{"auto", "av1", "qsv", "av1_qsv"},
		{"qsv", "av1", "qsv", "av1_qsv"},
		{"vaapi", "av1", "none", "libsvtav1"},
		{"missing", "av1", "none", "libsvtav1"},
	}
	for _, test := range tests {
		accelerator, encoder := capabilities.ResolveEncoder(test.configured, test.codec)
		if accelerator != test.accelerator || encoder != test.encoder {
			t.Errorf("ResolveEncoder(%q, %q) = %q/%q", test.configured, test.codec, accelerator, encoder)
		}
	}
	softwareOnly := Capabilities{Backends: capabilities.Backends[:1], Probed: true}
	if accelerator, encoder := softwareOnly.ResolveEncoder("auto", "h264"); accelerator != "none" || encoder != "libx264" {
		t.Fatalf("automatic software fallback = %q/%q", accelerator, encoder)
	}
	unprobed := Capabilities{Selected: "qsv"}
	if accelerator, encoder := unprobed.ResolveEncoder("auto", ""); accelerator != "qsv" || encoder != "h264_qsv" {
		t.Fatalf("unprobed automatic encoder = %q/%q", accelerator, encoder)
	}
	if !unprobed.SupportsCodec("hevc") || unprobed.SupportsCodec("vvc") {
		t.Fatal("unprobed codec validation changed")
	}
}

func TestPreferredCodecUsesClientAndHardwareEvidence(t *testing.T) {
	t.Parallel()
	hardware := Capabilities{
		Backends: []Backend{
			{ID: "none", Usable: true, encoders: map[string]string{"h264": "libx264", "av1": "libsvtav1", "hevc": "libx265", "vp9": "libvpx-vp9"}},
			{ID: "qsv", Usable: true, encoders: map[string]string{"av1": "av1_qsv", "hevc": "hevc_qsv", "vp9": "vp9_qsv"}},
		},
		Selected: "qsv",
		Probed:   true,
	}
	tests := []struct {
		configured, accelerator string
		client                  []string
		want                    string
	}{
		{"hevc", "auto", []string{"h264"}, "hevc"},
		{"auto", "auto", nil, "h264"},
		{"auto", "auto", []string{"av1", "hevc", "vp9", "h264"}, "hevc"},
		{"auto", "auto", []string{"hevc", "vp9", "h264"}, "hevc"},
		{"auto", "auto", []string{"vp9", "h264"}, "vp9"},
		{"auto", "none", []string{"av1", "hevc", "vp9"}, ""},
	}
	for _, test := range tests {
		if got := hardware.PreferredCodec(test.configured, test.accelerator, test.client); got != test.want {
			t.Errorf("PreferredCodec(%q, %q, %v) = %q, want %q", test.configured, test.accelerator, test.client, got, test.want)
		}
	}
	hardware.Probed = false
	if got := hardware.PreferredCodec("auto", "auto", []string{"av1"}); got != "h264" {
		t.Fatalf("unprobed codec = %q", got)
	}
}

func TestCodecCapabilitiesPreservePlayerPolicy(t *testing.T) {
	t.Parallel()
	hardware := Capabilities{Backends: []Backend{
		{Usable: true, encoders: map[string]string{"h264": "libx264", "hevc": "libx265", "vvc": "libvvenc"}},
		{encoders: map[string]string{"av1": "av1_qsv"}},
	}}
	got := codecCapabilities(hardware)
	if len(got) != 6 || got[0].ID != "h264" || !got[0].Usable || got[2].ID != "av1" || got[2].Usable {
		t.Fatalf("codec capabilities = %#v", got)
	}
}

func TestHLSCodecsPreservePlayerManifestValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		video, audio, mode, output string
		hasAudio                   bool
		want                       string
	}{
		{"h264", "aac", "remux", "", true, "avc1.64002a,mp4a.40.2"},
		{"hevc", "mp3", "remux", "", true, "hvc1,mp4a.6B"},
		{"vp9", "ac3", "remux", "", true, "vp09,ac-3"},
		{"av1", "eac3", "remux", "", true, "av01,ec-3"},
		{"unknown", "unknown", "transcode", "hevc", true, "hvc1,mp4a.40.2"},
		{"unknown", "aac", "direct", "av1", true, "av01,mp4a.40.2"},
		{"h264", "aac", "remux", "", false, "avc1.64002a"},
	}
	for _, test := range tests {
		if got := HLSCodecs(test.video, test.audio, test.mode, test.output, test.hasAudio); got != test.want {
			t.Errorf("HLSCodecs(%q, %q, %q, %q, %v) = %q, want %q", test.video, test.audio, test.mode, test.output, test.hasAudio, got, test.want)
		}
	}
}

func TestEncoderForIsReadOnly(t *testing.T) {
	t.Parallel()
	backend := Backend{encoders: map[string]string{"h264": "libx264"}}
	if got := backend.EncoderFor("h264"); got != "libx264" {
		t.Fatalf("encoder = %q", got)
	}
	if got := backend.EncoderFor("unknown"); !reflect.DeepEqual(got, "") {
		t.Fatalf("unknown encoder = %q", got)
	}
}
