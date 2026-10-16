package transcodehardware

import (
	"strings"
	"testing"
	"time"
)

func TestBackendPresentationCoversEveryState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		backend        Backend
		name, status   string
		reasonContains string
	}{
		{verifiedPresentationBackend("none"), "Processor", "Smoke check passed", "smoke check passed"},
		{Backend{ID: "none"}, "Processor", "Other system", "working video tools"},
		{verifiedPresentationBackend("qsv"), "Intel graphics", "Smoke check passed", "smoke check passed"},
		{Backend{ID: "cuda", Detected: true}, "NVIDIA graphics", "Needs setup", "no encoding operation"},
		{Backend{ID: "asahi"}, "Apple Silicon", "Processor only", "Asahi Linux"},
		{Backend{ID: "vaapi", Supported: true}, "AMD or Intel graphics", "Not available", "installed video tools"},
		{Backend{ID: "rkmpp"}, "Rockchip graphics", "Other system", "different system"},
		{Backend{ID: "v4l2m2m"}, "Built-in Linux video hardware", "Other system", "different system"},
		{Backend{ID: "videotoolbox"}, "Apple graphics", "Other system", "different system"},
		{Backend{ID: "amf"}, "AMD graphics", "Other system", "different system"},
		{Backend{ID: "mf"}, "Windows video hardware", "Other system", "different system"},
		{Backend{ID: "future", Name: "Future"}, "Future", "Other system", "different system"},
	}
	for _, test := range tests {
		if got := test.backend.FriendlyName(); got != test.name {
			t.Errorf("FriendlyName(%q) = %q, want %q", test.backend.ID, got, test.name)
		}
		if got := test.backend.SimpleStatus(); got != test.status {
			t.Errorf("SimpleStatus(%q) = %q, want %q", test.backend.ID, got, test.status)
		}
		if got := test.backend.SimpleReason(); !strings.Contains(got, test.reasonContains) {
			t.Errorf("SimpleReason(%q) = %q", test.backend.ID, got)
		}
	}
}

func TestCodecPresentationCoversEveryState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		codec          Codec
		status, reason string
	}{
		{Codec{ID: "h264", Supported: true, Usable: true}, "Recommended", "Works with the widest"},
		{Codec{ID: "hevc", Supported: true, Usable: true}, "Available", "Can save bandwidth"},
		{Codec{ID: "av1", Supported: true, Usable: true}, "Available", "Can save more bandwidth"},
		{Codec{ID: "vp9", Supported: true, Usable: true}, "Available", "compatible browsers"},
		{Codec{ID: "h264", Supported: true}, "Not available", "installed video tools"},
		{Codec{ID: "vvc"}, "Not ready yet", "playback support is limited"},
		{Codec{ID: "future", Supported: true, Usable: true}, "Available", ""},
	}
	for _, test := range tests {
		if got := test.codec.SimpleStatus(); got != test.status {
			t.Errorf("SimpleStatus(%q) = %q, want %q", test.codec.ID, got, test.status)
		}
		if got := test.codec.SimpleReason(); !strings.Contains(got, test.reason) {
			t.Errorf("SimpleReason(%q) = %q", test.codec.ID, got)
		}
	}
}

func verifiedPresentationBackend(id string) Backend {
	state := &verification{operations: map[string][]Operation{id: {{Codec: "h264", Status: "passed"}}}}
	return Backend{ID: id, Usable: true, verification: state, encoders: map[string]string{"h264": "encoder"}}
}

func TestDecodeSummaryListsVerifiedInputsInPriorityOrder(t *testing.T) {
	state := &verification{operations: map[string][]Operation{"cuda": {
		{Codec: "h264", Device: "0", Status: "passed", DecodeInputs: []string{"av110", "h264"}},
		{Codec: "h264", Device: "1", Status: "passed", DecodeInputs: []string{"hevc10"}},
		{Codec: "h264", Device: "0", Status: "passed", HardwareToneMap: "cuda", ToneMapInput: "hdr10", DecodeInputs: []string{"vp9"}},
	}}, failed: map[string]time.Time{}}
	backend := Backend{ID: "cuda", verification: state, encoders: map[string]string{"h264": "h264_nvenc"}}
	if got := backend.DecodeSummary(); got != "AVC / H.264, HEVC / H.265 10-bit, AV1 10-bit. Other inputs decode on the processor." {
		t.Fatalf("decode summary = %q", got)
	}
	unverified := Backend{ID: "vaapi", verification: &verification{operations: map[string][]Operation{}, failed: map[string]time.Time{}}}
	if got := unverified.DecodeSummary(); !strings.HasPrefix(got, "Not verified.") {
		t.Fatalf("unverified summary = %q", got)
	}
	if (Backend{ID: "none", verification: state}).DecodeSummary() != "" || (Backend{ID: "cuda"}).DecodeSummary() != "" {
		t.Fatal("software or undetected backends reported decoding")
	}
	page := EnhanceSettingsPage(legacyTranscoderSupportHTML)
	if !strings.Contains(page, "{{if .DecodeSummary}}<p><strong>Hardware decoding:</strong> {{.DecodeSummary}}</p>{{end}}") {
		t.Fatal("settings page lacks the decode summary")
	}
}
