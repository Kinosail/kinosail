package transcodehardware

import (
	"context"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func toneMapFixture() Capabilities {
	state := &verification{operations: map[string][]Operation{
		"cuda": {
			{Codec: "h264", Device: "0", Status: "passed"},
			{Codec: "h264", Device: "0", Status: "passed", HardwareToneMap: "cuda", ToneMapInput: "hdr10"},
			{Codec: "h264", Device: "0", Status: "passed", HardwareToneMap: "cuda", ToneMapInput: "hdr10", ToneMapSoftwareFrames: true},
			{Codec: "h264", Device: "0", Status: "passed", HardwareToneMap: "cuda", ToneMapInput: "hlg"},
		},
		"none": {{Codec: "h264", Status: "passed"}},
	}, failed: map[string]time.Time{}}
	return Capabilities{Probed: true, verification: state, Backends: []Backend{
		{ID: "cuda", verification: state, encoders: map[string]string{"h264": "h264_nvenc"}},
		{ID: "none", verification: state, encoders: map[string]string{"h264": "libx264"}},
	}}
}

func TestToneMappingRequiresMatchingDeviceCodecHDRAndFramePath(t *testing.T) {
	hardware := toneMapFixture()
	base := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", ToneMap: true, ToneMapInput: "hdr10"}
	for _, test := range []struct {
		name   string
		change func(*transcodepolicy.Settings)
		want   string
	}{
		{"HDR10", func(*transcodepolicy.Settings) {}, "cuda"},
		{"HLG", func(s *transcodepolicy.Settings) { s.ToneMapInput = "hlg" }, "cuda"},
		{"subtitle path", func(s *transcodepolicy.Settings) { s.SoftwareFilters = true }, "cuda"},
		{"wrong device", func(s *transcodepolicy.Settings) { s.Device = "1" }, ""},
		{"wrong codec", func(s *transcodepolicy.Settings) { s.Codec = "hevc" }, ""},
		{"unsupported HDR", func(s *transcodepolicy.Settings) { s.ToneMapInput = "dolby-vision" }, ""},
		{"unknown HDR", func(s *transcodepolicy.Settings) { s.ToneMapInput = "unknown" }, ""},
		{"SDR", func(s *transcodepolicy.Settings) { s.ToneMap = false }, ""},
		{"fallback", func(s *transcodepolicy.Settings) { s.DisableHardwareToneMap = true }, ""},
		{"untested HLG subtitles", func(s *transcodepolicy.Settings) { s.ToneMapInput = "hlg"; s.SoftwareFilters = true }, ""},
	} {
		options := base
		test.change(&options)
		actual, err := hardware.ColorSettings(options)
		if err != nil || actual.HardwareToneMap != test.want {
			t.Fatalf("%s: %#v %v", test.name, actual, err)
		}
	}
	unverified := Capabilities{}
	actual, _ := unverified.ColorSettings(base)
	if actual.HardwareToneMap != "" {
		t.Fatal("missing evidence enabled hardware tone mapping")
	}
}

func TestToneMapFailureKeepsEncoderAndOtherFormats(t *testing.T) {
	hardware := toneMapFixture()
	options, _ := hardware.ColorSettings(transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", ToneMap: true, ToneMapInput: "hdr10", Cache: "same-policy"})
	candidates := hardware.Recovery(options)
	if len(candidates) != 2 || candidates[0].Accelerator != "cuda" || candidates[0].HardwareToneMap != "" || !candidates[0].DisableHardwareToneMap || !candidates[0].ToneMap || candidates[0].Cache != options.Cache {
		t.Fatalf("recovery lost encoder or color policy: %#v", candidates)
	}
	hardware.RecordProcessingFailure(options)
	if hardware.Backend("cuda").EncoderFor("h264") == "" {
		t.Fatal("tone map failure disabled encoder")
	}
	if next, _ := hardware.ColorSettings(options); next.HardwareToneMap != "" {
		t.Fatal("failed tone map operation remained enabled")
	}
	options.ToneMapInput = "hlg"
	if next, _ := hardware.ColorSettings(options); next.HardwareToneMap != "cuda" {
		t.Fatal("HDR10 failure disabled independently checked HLG")
	}
	for _, candidate := range candidates {
		next, _ := hardware.ColorSettings(candidate)
		if next.HardwareToneMap != "" || next.Codec != "h264" || !next.ToneMap {
			t.Fatalf("fallback re-enabled failed processing: %#v", next)
		}
	}
}

func TestToneMapStartupDoesNotReuseSDRResults(t *testing.T) {
	backend := Backend{ID: "cuda", encoders: map[string]string{"h264": "h264_nvenc"}}
	baseline := Operation{Codec: "h264", Device: "0", Status: "passed"}
	settings := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", Encoder: "h264_nvenc"}
	for _, specific := range []bool{false, true} {
		state := &verification{operations: map[string][]Operation{}, failed: map[string]time.Time{}}
		calls := 0
		state.verifyToneMapping(t.Context(), ProbeOptions{FFmpeg: "test-ffmpeg"}, &backend, baseline, settings, func(_ context.Context, _ string, options transcodepolicy.Settings, _ string) transcodepolicy.CheckResult {
			calls++
			result := transcodepolicy.CheckResult{Status: "passed"}
			if specific {
				result.HardwareToneMap = options.HardwareToneMap
				result.ToneMapInput = options.ToneMapInput
			}
			return result
		})
		if calls != 4 {
			t.Fatalf("expected independent HDR and frame-path checks, got %d", calls)
		}
		want := 0
		if specific {
			want = 4
		}
		if len(state.operations["cuda"]) != want {
			t.Fatalf("SDR results certified HDR: %#v", state.operations)
		}
	}
}

func TestToneMapFailureClassificationPreservesStorageErrors(t *testing.T) {
	for _, detail := range []string{"tonemap_cuda failed to configure output", "tonemap_opencl kernel failed", "vpp_qsv initialization failed"} {
		if !HardwareFailure(detail) {
			t.Fatalf("missed hardware failure: %s", detail)
		}
		if HardwareFailure("No space left: " + detail) {
			t.Fatalf("storage error authorized retry: %s", detail)
		}
	}
}

func TestOptionalToneMapFailureCannotConsumeCodecVerificationBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	discovered := Capabilities{Probed: true, Backends: []Backend{{ID: "cuda", Usable: true, encoders: map[string]string{"h264": "h264_nvenc", "hevc": "hevc_nvenc", "av1": "av1_nvenc"}}}}
	seen := map[string]bool{}
	actual := verify(ctx, ProbeOptions{Enabled: true, FFmpeg: "test-ffmpeg", Devices: []string{t.TempDir()}}, discovered, func(_ context.Context, _ string, options transcodepolicy.Settings, _ string) transcodepolicy.CheckResult {
		if options.HardwareToneMap != "" {
			if !seen["hevc"] || !seen["av1"] {
				t.Fatal("optional tone map ran before codec baselines")
			}
			cancel()
			return transcodepolicy.CheckResult{Status: "failed"}
		}
		seen[options.Codec] = true
		return transcodepolicy.CheckResult{Status: "passed", HardwareDecode: options.HardwareDecode}
	})
	if !actual.SupportsCodec("hevc") || !actual.SupportsCodec("av1") {
		t.Fatal("optional tone map failure removed codec support")
	}
}

func TestRockchipToneMapEvidenceUsesAccessibleRenderDevice(t *testing.T) {
	devices := backendDevices(Backend{ID: "rkmpp"}, []string{"/dev/dri/renderD129"})
	if len(devices) != 1 || devices[0] != "/dev/dri/renderD129" {
		t.Fatalf("lost tone mapping device identity: %v", devices)
	}
}
