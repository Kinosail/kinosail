package transcodehardware

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestVerificationDoesNotPromoteAnEncoderListing(t *testing.T) {
	discovered := Capabilities{Probed: true, Selected: "qsv", Backends: []Backend{
		{ID: "none", Name: "Software", Usable: true, encoders: map[string]string{"h264": "libx264"}},
		{ID: "qsv", Name: "Intel", Usable: true, encoders: map[string]string{"h264": "h264_qsv", "av1": "av1_qsv"}},
		{ID: "vaapi", Name: "AMD", Usable: true, encoders: map[string]string{"h264": "h264_vaapi"}},
	}}
	var order []string
	actual := verify(t.Context(), ProbeOptions{Enabled: true, FFmpeg: "test-ffmpeg", Devices: []string{t.TempDir()}}, discovered, func(_ context.Context, _ string, options transcodepolicy.Settings, _ string) transcodepolicy.CheckResult {
		order = append(order, options.Accelerator+":"+options.Codec)
		status := "failed"
		if options.Accelerator == "none" || options.Accelerator == "vaapi" {
			status = "passed"
		}
		return transcodepolicy.CheckResult{Status: status}
	})
	if actual.Selected != "vaapi" || actual.Backend("qsv").EncoderFor("av1") != "" || actual.SupportsCodec("av1") {
		t.Fatalf("unsupported hardware was selected: %#v", actual)
	}
	if actual.PreferredCodec("auto", "auto", []string{"av1", "h264"}) != "h264" {
		t.Fatal("automatic ignored verified H.264")
	}
	if len(order) < 3 || strings.Join(order[:3], ",") != "none:h264,qsv:h264,vaapi:h264" {
		t.Fatalf("baseline was not prioritized: %v", order)
	}
}

func TestPreferredDecodeIsCheckedBeforeOptionalCodecBudgetExpires(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	discovered := Capabilities{Probed: true, Backends: []Backend{
		{ID: "none", Usable: true, encoders: map[string]string{"h264": "libx264", "hevc": "libx265"}},
		{ID: "cuda", Usable: true, encoders: map[string]string{"h264": "h264_nvenc"}},
	}}
	decodes := 0
	actual := verify(ctx, ProbeOptions{Enabled: true, FFmpeg: "test-ffmpeg", Devices: []string{t.TempDir()}}, discovered, func(_ context.Context, _ string, options transcodepolicy.Settings, _ string) transcodepolicy.CheckResult {
		if options.Codec == "hevc" {
			cancel()
			return transcodepolicy.CheckResult{Status: "failed"}
		}
		if options.HardwareDecode {
			decodes++
		}
		return transcodepolicy.CheckResult{Status: "passed", HardwareDecode: options.HardwareDecode}
	})
	settings, err := actual.Settings(Selection{Accelerator: "auto"}, "h264")
	if err != nil || settings.Accelerator != "cuda" || !settings.HardwareDecode || decodes != 1 {
		t.Fatalf("preferred frame path lost its startup budget: %#v, %v, checks=%d", settings, err, decodes)
	}
	if actual.SupportsCodec("hevc") {
		t.Fatal("unfinished optional check enabled HEVC")
	}
}

func TestFailedDeviceEvidenceIsSharedAndExpires(t *testing.T) {
	state := &verification{operations: map[string][]Operation{"cuda": {{Codec: "h264", Device: "0", Status: "passed"}}}, failed: map[string]time.Time{}}
	hardware := Capabilities{Probed: true, verification: state, Backends: []Backend{{ID: "cuda", Usable: true, verification: state, encoders: map[string]string{"h264": "h264_nvenc"}}}}
	options := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Device: "0"}
	if hardware.Backend("cuda").EncoderFor("h264") == "" {
		t.Fatal("missing initial evidence")
	}
	hardware.RecordFailure(options)
	if hardware.SupportsCodec("h264") || hardware.Backend("cuda").currentlyUsable() {
		t.Fatal("failed operation remained selectable")
	}
	hardware.RecordCheck(options, transcodepolicy.CheckResult{Status: "passed", HardwareDecode: true})
	if hardware.Backend("cuda").EncoderFor("h264") == "" {
		t.Fatal("explicit successful check did not restore evidence")
	}
	state.failed[operationKey("cuda", "0", "h264")] = time.Now().Add(-time.Second)
	if !hardware.SupportsCodec("h264") {
		t.Fatal("expired failure did not permit recovery")
	}
}

func TestDecodeRecoveryPreservesEncodingAndPolicyCache(t *testing.T) { //nolint:cyclop // Recovery and subsequent cache reuse must share the same hardware evidence.
	state := &verification{operations: map[string][]Operation{
		"cuda": {{Codec: "h264", Device: "0", DecodeH264: true, Status: "passed"}},
		"none": {{Codec: "h264", Status: "passed"}},
	}, failed: map[string]time.Time{}}
	hardware := Capabilities{Probed: true, verification: state, Backends: []Backend{
		{ID: "none", Usable: true, verification: state, encoders: map[string]string{"h264": "libx264"}},
		{ID: "cuda", Usable: true, verification: state, encoders: map[string]string{"h264": "h264_nvenc"}},
	}}
	selection := Selection{Accelerator: "auto"}
	initial, err := hardware.Settings(selection, "h264")
	if err != nil || !initial.HardwareDecode || initial.Accelerator != "cuda" {
		t.Fatalf("initial hardware settings = %#v, %v", initial, err)
	}
	hardware.RecordDecodeFailure(initial)
	encodeOnly, err := hardware.Settings(selection, "h264")
	if err != nil || encodeOnly.HardwareDecode || encodeOnly.Accelerator != "cuda" || encodeOnly.Cache != initial.Cache {
		t.Fatalf("decode failure discarded encoding: %#v, %v", encodeOnly, err)
	}
	hardware.RecordFailure(encodeOnly)
	software, err := hardware.Settings(selection, "h264")
	if err != nil || software.Accelerator != "none" || software.Cache != initial.Cache {
		t.Fatalf("software recovery changed playback cache: %#v, %v", software, err)
	}
	for _, changed := range []Selection{{Accelerator: "none"}, {Accelerator: "auto", Transcoder: "quality"}, {Accelerator: "auto", ToneMap: true}} {
		next, err := hardware.Settings(changed, "h264")
		if err != nil || next.Cache == initial.Cache {
			t.Fatalf("changed policy reused stale cache: %#v, %v", next, err)
		}
	}
}

func TestRecoveryClassifiesFailureAndNeverChangesNegotiatedCodec(t *testing.T) { //nolint:cyclop // Each failure case checks classification and the original codec contract.
	for _, detail := range []string{"No space left on device", "Permission denied", "Invalid data found when processing input", "Error opening input: device setup failed", "network timeout"} {
		if HardwareFailure(detail) {
			t.Fatalf("unrelated failure authorized a hardware retry: %s", detail)
		}
	}
	if !HardwareFailure("Failed setup for format cuda: hwaccel initialisation returned error") {
		t.Fatal("hardware decode failure was not recognized")
	}
	hardware := Capabilities{Backends: []Backend{{ID: "none", Usable: true, encoders: map[string]string{"h264": "libx264", "av1": "libsvtav1"}}, {ID: "vaapi", Usable: true, encoders: map[string]string{"h264": "h264_vaapi", "av1": "av1_vaapi"}}}}
	candidates := hardware.Recovery(transcodepolicy.Settings{Codec: "av1", Accelerator: "cuda", HardwareDecode: true})
	if len(candidates) != 2 || candidates[0].Accelerator != "cuda" || candidates[0].HardwareDecode || candidates[1].Accelerator != "vaapi" {
		t.Fatalf("recovery = %#v", candidates)
	}
	for _, candidate := range candidates {
		if candidate.Codec != "av1" || candidate.Accelerator == "none" {
			t.Fatal("expensive software conversion or codec change escaped negotiation")
		}
	}
	if DeviceKey(transcodepolicy.Settings{Accelerator: "qsv", Device: "/dev/dri/renderD128"}) != DeviceKey(transcodepolicy.Settings{Accelerator: "vaapi", Device: "/dev/dri/renderD128"}) {
		t.Fatal("APIs for one device used independent session budgets")
	}
}

func TestChangedExecutableInvalidatesEvidenceUntilExplicitCheck(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(executable, []byte("old"), 0o700); err != nil { //nolint:gosec // This private temporary fixture must be executable by the test.
		t.Fatal(err)
	}
	state := &verification{ffmpeg: executable, operations: map[string][]Operation{}, failed: map[string]time.Time{}}
	hardware := Capabilities{Probed: true, verification: state, Backends: []Backend{{ID: "none", Supported: true, Detected: true, verification: state, encoders: map[string]string{"h264": "libx264"}}}}
	options := transcodepolicy.Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264"}
	hardware.RecordCheck(options, transcodepolicy.CheckResult{Status: "passed"})
	if !hardware.SupportsCodec("h264") {
		t.Fatal("successful explicit check did not enable a previously unavailable operation")
	}
	if err := os.WriteFile(executable, []byte("different executable"), 0o700); err != nil { //nolint:gosec // This private temporary fixture must be executable by the test.
		t.Fatal(err)
	}
	if hardware.SupportsCodec("h264") {
		t.Fatal("executable replacement retained old evidence")
	}
	hardware.RecordCheck(options, transcodepolicy.CheckResult{Status: "passed"})
	if !hardware.SupportsCodec("h264") {
		t.Fatal("new successful check did not replace stale identity")
	}
}
