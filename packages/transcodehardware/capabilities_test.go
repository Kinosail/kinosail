package transcodehardware

import (
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
