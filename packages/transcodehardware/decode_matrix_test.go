package transcodehardware

import (
	"context"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func decodeMatrixCapabilities(t *testing.T, decodes map[string]bool, hardwareFrameToneMap bool) Capabilities {
	t.Helper()
	discovered := Capabilities{Probed: true, Backends: []Backend{
		{ID: "none", Usable: true, encoders: map[string]string{"h264": "libx264"}},
		{ID: "vaapi", Usable: true, encoders: map[string]string{"h264": "h264_vaapi"}},
	}}
	var mu sync.Mutex
	return verify(t.Context(), ProbeOptions{Enabled: true, FFmpeg: "test-ffmpeg", Devices: []string{t.TempDir()}}, discovered, func(_ context.Context, _ string, options transcodepolicy.Settings, _ string) transcodepolicy.CheckResult {
		mu.Lock()
		defer mu.Unlock()
		input := options.DecodeInput
		if input == "" {
			input = "h264"
		}
		passed := !options.HardwareDecode || decodes[input]
		if options.HardwareToneMap != "" {
			passed = options.HardwareToneMap == "vaapi" && options.ToneMapInput == "hdr10" && (!options.HardwareDecode || hardwareFrameToneMap && input == "hevc10")
		}
		if !passed {
			return transcodepolicy.CheckResult{Status: "failed"}
		}
		return transcodepolicy.CheckResult{Status: "passed", HardwareDecode: options.HardwareDecode, HardwareToneMap: options.HardwareToneMap, ToneMapInput: options.ToneMapInput}
	})
}

func TestHardwareDecodeIsUsedOnlyForVerifiedInputs(t *testing.T) {
	capabilities := decodeMatrixCapabilities(t, map[string]bool{"h264": true, "hevc10": true}, false)
	settings, err := capabilities.Settings(Selection{Transcoder: "automatic", Accelerator: "auto"}, "h264")
	if err != nil || settings.Accelerator != "vaapi" || !settings.HardwareDecode || !settings.HardwareDecodeHEVC10 || settings.HardwareDecodeHEVC || settings.HardwareDecodeAV1 {
		t.Fatalf("verified decode inputs = %#v, %v", settings, err)
	}
	failed := settings
	failed.HardwareDecode, failed.DecodeInput = true, "hevc10"
	capabilities.RecordProcessingFailure(failed)
	settings, err = capabilities.Settings(Selection{Transcoder: "automatic", Accelerator: "auto"}, "h264")
	if err != nil || !settings.HardwareDecode || settings.HardwareDecodeHEVC10 {
		t.Fatalf("10-bit HEVC decode failure changed other evidence: %#v, %v", settings, err)
	}
	unverified := decodeMatrixCapabilities(t, map[string]bool{"h264": true}, false)
	settings, err = unverified.Settings(Selection{Transcoder: "automatic", Accelerator: "auto"}, "h264")
	if err != nil || !settings.HardwareDecode || settings.HardwareDecodeHEVC || settings.HardwareDecodeHEVC10 || settings.HardwareDecodeAV1 {
		t.Fatalf("unverified inputs used hardware decode: %#v, %v", settings, err)
	}
	none := decodeMatrixCapabilities(t, map[string]bool{"hevc": true, "hevc10": true, "av1": true}, false)
	if settings, _ = none.Settings(Selection{Transcoder: "automatic", Accelerator: "auto"}, "h264"); settings.HardwareDecode || settings.HardwareDecodeHEVC || settings.HardwareDecodeAV1 {
		t.Fatalf("extra inputs were trusted without a working hardware frame path: %#v", settings)
	}
}

func TestHardwareFrameToneMappingRequiresItsOwnEvidence(t *testing.T) {
	for _, verified := range []bool{false, true} {
		capabilities := decodeMatrixCapabilities(t, map[string]bool{"h264": true, "hevc10": true}, verified)
		options, err := capabilities.Settings(Selection{Transcoder: "automatic", Accelerator: "auto", ToneMap: true}, "h264")
		if err != nil {
			t.Fatal(err)
		}
		options.ToneMap, options.ToneMapInput, options.DecodeInput, options.HardwareDecode = true, "hdr10", "hevc10", false
		colored, err := capabilities.ColorSettings(options)
		if err != nil || colored.HardwareToneMap != "vaapi" || colored.HardwareDecode != verified || transcodepolicy.ToneMapHardwareFrames(colored) != verified {
			t.Fatalf("verified=%t tone map = %#v, %v", verified, colored, err)
		}
		options.DecodeInput = "hevc"
		if colored, _ = capabilities.ColorSettings(options); colored.HardwareDecode || colored.HardwareToneMap != "vaapi" {
			t.Fatalf("8-bit input used hardware-frame tone mapping: %#v", colored)
		}
	}
}
