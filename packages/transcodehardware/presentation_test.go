package transcodehardware

import (
	"strings"
	"testing"
)

func TestEnhanceSettingsPagePreservesPlayerCopy(t *testing.T) {
	t.Parallel()
	legacy := legacyHardwareOptionsHTML + legacyTranscoderSupportHTML + strings.Join([]string{
		`<h2>Transcoder</h2><p>Automatic uses H.264 for broad device support. Choose a newer codec only when every playback device supports it.</p>`,
		`<label>Quality <select name="quality">`,
		`<label>Video codec <select name="codec">`,
		`>Automatic · H.264 ({{t "Recommended"}})</option>`,
		`<label>Hardware <select name="accelerator"><option value="none" {{if eq .Accelerator "none"}}selected{{end}}>Software</option>`,
		`</select></label><label><input type="checkbox" name="toneMap"`,
		`> Tone-map HDR to SDR</label>`,
		`Local transcoder test`,
		`Transcoding is ready`,
		`Kinosail encoded a local test clip with`,
		`This confirms FFmpeg, scaling, {{.TranscoderTest.CodecName}}, AAC, and the selected encoder. No Library Content is used. Actual HDR, subtitles, networks, and playback devices can still vary.`,
		`Transcoder test failed`,
		`Choose Automatic or Software, save, and test again. If hardware is selected, make sure its GPU device is available to the Server container.`,
		`Is local transcoding ready?`,
		`Create a tiny synthetic clip on this Server to check FFmpeg, scaling, {{.TranscoderTest.CodecName}}, AAC, and <strong>{{.TranscoderTest.Backend}}</strong>. No Library Content is used.`,
		`Run local test`,
		`Test again`,
	}, "|")
	page := EnhanceSettingsPage(legacy)
	for _, expected := range []string{
		hardwareOptionsHTML, transcoderSupportHTML, "Video conversion", "Conversion preference", "Video format", "Automatic per device", "Speed up with", "Processor", "Automatic will use:", "Improve HDR colors", "Quick compatibility check", "Video conversion is ready", "Kinosail converted a small test video", "The basic video tools work", "Video check failed", "Choose Automatic or Processor", "Can this Server convert video?", "Kinosail makes a small test video", "Run quick check", "Check again",
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("enhanced page lacks %q", expected)
		}
	}
	if EnhanceSettingsPage("unchanged") != "unchanged" {
		t.Fatal("unrelated settings copy changed")
	}
}

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
