package transcodehardware

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func decodeCheck(decodable map[string]bool, order *[]string) func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult {
	return func(_ context.Context, _ string, options transcodepolicy.Settings, _ string) transcodepolicy.CheckResult {
		label := options.Accelerator + ":" + options.Codec
		if options.OutputHDR != "" {
			label += ":" + options.OutputHDR
		}
		if options.HardwareToneMap != "" {
			label += ":tonemap=" + options.HardwareToneMap + "/" + options.ToneMapInput
		}
		if options.HardwareDecode {
			label += ":decode=" + options.DecodeInput
		}
		if order != nil {
			*order = append(*order, label)
		}
		if options.HardwareDecode && !decodable[options.DecodeInput] {
			return transcodepolicy.CheckResult{Status: "failed"}
		}
		return transcodepolicy.CheckResult{Status: "passed", HardwareDecode: options.HardwareDecode, DecodeInput: options.DecodeInput, HardwareToneMap: options.HardwareToneMap, ToneMapInput: options.ToneMapInput, OutputHDR: options.OutputHDR}
	}
}

func TestStartupVerifiesEveryDecodeInputPerDeviceAndSharesItAcrossEncoders(t *testing.T) {
	discovered := Capabilities{Probed: true, Backends: []Backend{
		{ID: "none", Usable: true, encoders: map[string]string{"h264": "libx264", "hevc": "libx265"}},
		{ID: "cuda", Usable: true, encoders: map[string]string{"h264": "h264_nvenc", "hevc": "hevc_nvenc"}},
	}}
	var order []string
	actual := verify(t.Context(), ProbeOptions{Enabled: true, FFmpeg: "test-ffmpeg", Devices: []string{t.TempDir()}}, discovered, decodeCheck(map[string]bool{"h264": true, "hevc10": true, "hevc": true, "av1": true}, &order))
	joined := strings.Join(order, ",")
	if strings.Contains(joined, "none:h264:decode") || strings.Index(joined, "cuda:h264:decode=h264") > strings.Index(joined, "cuda:hevc,") || strings.Index(joined, "cuda:h264:decode=hevc10") < strings.Index(joined, "cuda:hevc,") {
		t.Fatalf("decode checks ran for software or out of priority order: %v", order)
	}
	if strings.Index(joined, "decode=hevc10") > strings.Index(joined, "decode=hevc,") || strings.Index(joined, "decode=av110") > strings.Index(joined, "decode=av1,") || !strings.Contains(joined, "decode=vp910") {
		t.Fatalf("decode inputs lost their priority: %v", order)
	}
	for _, codec := range []string{"h264", "hevc"} {
		settings, err := actual.Settings(Selection{Accelerator: "auto"}, codec)
		if err != nil || settings.Accelerator != "cuda" || !slices.Equal(settings.HardwareDecodeInputs, []string{"h264", "hevc10", "hevc", "av1"}) {
			t.Fatalf("%s settings lost shared decode evidence: %#v, %v", codec, settings, err)
		}
	}
	if !strings.Contains(joined, "cuda:hevc:hdr10:decode=hevc10") || !strings.Contains(joined, "cuda:hevc:hlg:decode=hevc10") || !strings.Contains(joined, "cuda:h264:tonemap=cuda/hdr10:decode=hevc10") {
		t.Fatalf("hardware decoded HDR paths were not verified: %v", order)
	}
	hdr := transcodepolicy.Settings{Codec: "hevc", Accelerator: "cuda", OutputHDR: "hdr10", HardwareDecode: true, DecodeInput: "hevc10"}
	if actual, err := actual.ColorSettings(hdr); err != nil || !actual.HardwareDecode {
		t.Fatalf("HDR output lost verified hardware decode: %#v, %v", actual, err)
	}
	hdr.DecodeInput = "av110"
	if actual, err := actual.ColorSettings(hdr); err != nil || actual.HardwareDecode {
		t.Fatalf("HDR output claimed unverified decode input: %#v, %v", actual, err)
	}
}

func TestUnverifiedTenBitDecodeSkipsHardwareDecodedHDRChecks(t *testing.T) {
	discovered := Capabilities{Probed: true, Backends: []Backend{{ID: "vaapi", Usable: true, encoders: map[string]string{"h264": "h264_vaapi", "hevc": "hevc_vaapi"}}}}
	var order []string
	actual := verify(t.Context(), ProbeOptions{Enabled: true, FFmpeg: "test-ffmpeg", Devices: []string{t.TempDir()}}, discovered, decodeCheck(map[string]bool{"h264": true, "hevc": true}, &order))
	joined := strings.Join(order, ",")
	if strings.Contains(joined, "hdr10:decode") || strings.Contains(joined, "tonemap=vaapi/hdr10:decode") || !strings.Contains(joined, "vaapi:h264:decode=hevc10") {
		t.Fatalf("HDR decode checks ran without ten-bit evidence: %v", order)
	}
	settings, _ := actual.Settings(Selection{Accelerator: "auto"}, "hevc")
	if !slices.Equal(settings.HardwareDecodeInputs, []string{"h264", "hevc"}) {
		t.Fatalf("decode evidence = %v", settings.HardwareDecodeInputs)
	}
}

func hardwareDecodedToneMapFixture() Capabilities {
	state := &verification{operations: map[string][]Operation{
		"cuda": {
			{Codec: "h264", Device: "0", Status: "passed", DecodeInputs: []string{"h264", "hevc10"}},
			{Codec: "h264", Device: "0", Status: "passed", HardwareToneMap: "cuda", ToneMapInput: "hdr10", DecodeInputs: []string{"hevc10"}},
			{Codec: "h264", Device: "0", Status: "passed", HardwareToneMap: "cuda", ToneMapInput: "hlg"},
		},
		"none": {{Codec: "h264", Status: "passed"}},
	}, failed: map[string]time.Time{}}
	return Capabilities{Probed: true, verification: state, Backends: []Backend{
		{ID: "cuda", verification: state, encoders: map[string]string{"h264": "h264_nvenc"}},
		{ID: "none", verification: state, encoders: map[string]string{"h264": "libx264"}},
	}}
}

func TestToneMapSelectionUsesHardwareDecodeOnlyWithMatchingEvidence(t *testing.T) {
	hardware := hardwareDecodedToneMapFixture()
	base := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", ToneMap: true, ToneMapInput: "hdr10", HardwareDecode: true, DecodeInput: "hevc10"}
	for _, test := range []struct {
		name       string
		change     func(*transcodepolicy.Settings)
		wantMethod string
		wantDecode bool
	}{
		{"verified HDR10", func(*transcodepolicy.Settings) {}, "cuda", true},
		{"HLG without decode evidence", func(s *transcodepolicy.Settings) { s.ToneMapInput = "hlg" }, "cuda", false},
		{"other ten-bit input", func(s *transcodepolicy.Settings) { s.DecodeInput = "av110" }, "cuda", false},
		{"software filters", func(s *transcodepolicy.Settings) { s.SoftwareFilters = true }, "", false},
		{"hardware tone map disabled", func(s *transcodepolicy.Settings) { s.DisableHardwareToneMap = true }, "", false},
		{"unsupported HDR", func(s *transcodepolicy.Settings) { s.ToneMapInput = "dolby-vision" }, "", false},
	} {
		options := base
		test.change(&options)
		actual, err := hardware.ColorSettings(options)
		if err != nil || actual.HardwareToneMap != test.wantMethod || actual.HardwareDecode != test.wantDecode {
			t.Fatalf("%s: %#v %v", test.name, actual, err)
		}
	}
	sdr := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", HardwareDecode: true, DecodeInput: "hevc10"}
	if actual, _ := hardware.ColorSettings(sdr); !actual.HardwareDecode {
		t.Fatal("SDR decode evidence lost")
	}
	if actual, _ := (Capabilities{}).ColorSettings(base); actual.HardwareDecode {
		t.Fatal("missing evidence kept hardware decode for a tone-mapped source")
	}
}

func TestHardwareDecodedToneMapFailureFallsBackBeforeLeavingTheDevice(t *testing.T) {
	hardware := hardwareDecodedToneMapFixture()
	options, _ := hardware.ColorSettings(transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", ToneMap: true, ToneMapInput: "hdr10", HardwareDecode: true, DecodeInput: "hevc10", Cache: "policy"})
	if !options.HardwareDecode || options.HardwareToneMap != "cuda" {
		t.Fatalf("fixture did not select the hardware decoded tone map: %#v", options)
	}
	candidates := hardware.Recovery(options)
	if len(candidates) != 3 || candidates[0].HardwareDecode || candidates[0].HardwareToneMap != "cuda" || candidates[1].HardwareToneMap != "" || !candidates[1].DisableHardwareToneMap || candidates[2].Accelerator != "none" {
		t.Fatalf("recovery ladder = %#v", candidates)
	}
	for _, candidate := range candidates {
		if candidate.Cache != options.Cache || !candidate.ToneMap || candidate.Codec != "h264" {
			t.Fatalf("recovery changed policy: %#v", candidate)
		}
	}
	hardware.RecordProcessingFailure(options)
	next, _ := hardware.ColorSettings(options)
	if next.HardwareToneMap != "cuda" || next.HardwareDecode {
		t.Fatalf("decode failure removed the tone map or kept hardware decode: %#v", next)
	}
	sdr, _ := hardware.ColorSettings(transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", HardwareDecode: true, DecodeInput: "hevc10"})
	if !sdr.HardwareDecode {
		t.Fatal("tone map decode failure disabled SDR hardware decode")
	}
	hardware.RecordProcessingFailure(next)
	if final, _ := hardware.ColorSettings(options); final.HardwareToneMap != "" || final.HardwareDecode {
		t.Fatalf("software decode tone map failure kept hardware processing: %#v", final)
	}
	if hardware.Backend("cuda").EncoderFor("h264") == "" {
		t.Fatal("tone map failures disabled the encoder")
	}
}

func TestBaselineDecodeFailureOnlyRemovesTheFailedInput(t *testing.T) {
	hardware := hardwareDecodedToneMapFixture()
	options := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", HardwareDecode: true, DecodeInput: "hevc10"}
	hardware.RecordProcessingFailure(options)
	settings, _ := hardware.Settings(Selection{Accelerator: "auto"}, "h264")
	if !slices.Equal(settings.HardwareDecodeInputs, []string{"h264"}) {
		t.Fatalf("decode inputs = %v", settings.HardwareDecodeInputs)
	}
	toneMapped, _ := hardware.ColorSettings(transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0", ToneMap: true, ToneMapInput: "hdr10", HardwareDecode: true, DecodeInput: "hevc10"})
	if !toneMapped.HardwareDecode {
		t.Fatal("baseline decode failure removed separately verified tone map decode")
	}
	hardware.RecordCheck(options, transcodepolicy.CheckResult{Status: "passed", HardwareDecode: true, DecodeInput: "hevc10"})
	settings, _ = hardware.Settings(Selection{Accelerator: "auto"}, "h264")
	if !slices.Equal(settings.HardwareDecodeInputs, []string{"h264", "hevc10"}) {
		t.Fatalf("explicit check did not restore decode evidence: %v", settings.HardwareDecodeInputs)
	}
}

func TestDecodeFailureMarkersAuthorizeRecovery(t *testing.T) {
	for _, detail := range []string{"[Parsed_scale_vt_0 @ 0x1] Failed to configure output", "hwaccel initialisation returned error", "No decoder surfaces left", "Hardware accelerator failed to decode picture", "[Parsed_tonemap_vaapi_0 @ 0x1] Failed to initialise VAAPI"} {
		if !HardwareFailure(detail) {
			t.Errorf("missed hardware decode failure: %s", detail)
		}
	}
}
