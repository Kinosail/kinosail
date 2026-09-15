package transcodepolicy

import (
	"reflect"
	"strings"
	"testing"
)

func TestStableCodecsAndBackendsUsePlayerRecipes(t *testing.T) { //nolint:paralleltest // Subtests share no mutable state.
	tests := []struct {
		codec, accelerator, encoder string
		want                        []string
	}{
		{"h264", "none", "libx264", []string{"-c:v libx264", "-preset veryfast", "-crf 22"}},
		{"hevc", "qsv", "hevc_qsv", []string{"-c:v hevc_qsv", "-global_quality 22", "-tag:v hvc1"}},
		{"av1", "cuda", "av1_nvenc", []string{"-c:v av1_nvenc", "-cq 22", "-preset p4"}},
		{"av1", "none", "libsvtav1", []string{"-c:v libsvtav1", "-preset 8", "-crf 22"}},
		{"vp9", "vaapi", "vp9_vaapi", []string{"-c:v vp9_vaapi", "-qp 22"}},
		{"vp9", "none", "libvpx-vp9", []string{"-c:v libvpx-vp9", "-deadline realtime", "-cpu-used 6"}},
		{"hevc", "v4l2m2m", "hevc_v4l2m2m", []string{"-c:v hevc_v4l2m2m", "-tag:v hvc1"}},
		{"av1", "mf", "av1_mf", []string{"-c:v av1_mf", "-hw_encoding 1"}},
	}
	for _, test := range tests {
		t.Run(test.codec+"/"+test.accelerator, func(t *testing.T) {
			_, output := VideoArguments(Settings{Codec: test.codec, Accelerator: test.accelerator, Encoder: test.encoder, Preset: "veryfast", CRF: "22"}, "1280")
			arguments := strings.Join(output, " ")
			for _, expected := range test.want {
				if !strings.Contains(arguments, expected) {
					t.Errorf("arguments %q lack %q", arguments, expected)
				}
			}
		})
	}
}

func TestEveryPlayerHardwareBackendHasAnFFmpegRecipe(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"qsv":          {"-hwaccel qsv", "scale_qsv", "-c:v h264_qsv"},
		"cuda":         {"-hwaccel cuda", "scale_cuda", "-c:v h264_nvenc"},
		"vaapi":        {"-hwaccel vaapi", "scale_vaapi", "-c:v h264_vaapi"},
		"rkmpp":        {"-init_hw_device rkmpp=rk", "scale_rkrga", "-c:v h264_rkmpp"},
		"v4l2m2m":      {"scale=w=", "-c:v h264_v4l2m2m"},
		"videotoolbox": {"scale=w=", "-c:v h264_videotoolbox"},
		"amf":          {"scale=w=", "-c:v h264_amf"},
		"mf":           {"scale=w=", "-c:v h264_mf"},
	}
	for accelerator, expected := range tests {
		input, output := VideoArguments(Settings{Accelerator: accelerator, CRF: "22", Preset: "veryfast", HardwareDecode: true}, "1280")
		arguments := strings.Join(append(input, output...), " ")
		for _, value := range expected {
			if !strings.Contains(arguments, value) {
				t.Errorf("%s arguments %q lack %q", accelerator, arguments, value)
			}
		}
	}
}

func TestCodecCatalogIsIsolatedAndOrdered(t *testing.T) {
	t.Parallel()
	first := Codecs()
	first[0].Software[0] = "changed"
	first[0].Hardware["qsv"] = "changed"
	second := Codecs()
	if second[0].Software[0] != "libx264" || second[0].Hardware["qsv"] != "h264_qsv" {
		t.Fatalf("codec catalog was mutated: %#v", second[0])
	}
	if got := EncoderCodecIDs(map[string]string{"vp9": "x", "h264": "x"}); !reflect.DeepEqual(got, []string{"h264", "vp9"}) {
		t.Fatalf("codec order = %v", got)
	}
}

func TestCapabilitiesUsePlayerReadinessPolicy(t *testing.T) {
	t.Parallel()
	capabilities := Capabilities([]Backend{{Encoders: map[string]string{"h264": "libx264", "hevc": "libx265", "vvc": "libvvenc"}, Usable: true}, {Encoders: map[string]string{"av1": "av1_qsv"}}})
	byID := make(map[string]Capability, len(capabilities))
	for _, capability := range capabilities {
		byID[capability.ID] = capability
	}
	type summary struct {
		detected, supported, usable, hasAction bool
		status                                 string
	}
	tests := map[string]summary{
		"h264": {true, true, true, false, "Ready"},
		"av1":  {true, true, false, true, "Hardware setup needed"},
		"vvc":  {true, false, false, true, "Not ready for streaming"},
		"av2":  {false, false, false, true, "Not available yet"},
		"vp9":  {false, true, false, true, "FFmpeg update needed"},
	}
	for id, want := range tests {
		capability := byID[id]
		got := summary{capability.Detected, capability.Supported, capability.Usable, capability.Action != "", capability.Status}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s capability = %#v, want %#v", id, got, want)
		}
	}
}

func TestCodecAndCapabilityValidationFailsClosed(t *testing.T) {
	t.Parallel()
	for _, codec := range []string{"H264", "h264 ", "unknown", "vvc", "av2"} {
		if ValidCodec(codec) {
			t.Errorf("ValidCodec(%q) = true", codec)
		}
	}
	if HasCapability("h264_nvenc_extra", "h264_nvenc") {
		t.Fatal("partial FFmpeg capability matched")
	}
	if got := FirstEncoder("libaom-av1 libsvtav1", []string{"libsvtav1", "libaom-av1"}); got != "libsvtav1" {
		t.Fatalf("first encoder = %q", got)
	}
}

func TestToneMappingAndCompatibility(t *testing.T) {
	t.Parallel()
	input, output := VideoArguments(Settings{Accelerator: "rkmpp", Encoder: "h264_rkmpp", CRF: "22", Preset: "veryfast", ToneMap: true}, "1280")
	arguments := strings.Join(append(input, output...), " ")
	for _, expected := range []string{"zscale=t=linear", "tonemap=hable", "format=yuv420p", "scale=w=", "-c:v h264_rkmpp"} {
		if !strings.Contains(arguments, expected) {
			t.Errorf("tone-map arguments %q lack %q", arguments, expected)
		}
	}
	for _, forbidden := range []string{"-hwaccel rkmpp", "drm_prime", "scale_rkrga"} {
		if strings.Contains(arguments, forbidden) {
			t.Errorf("tone-map arguments %q include %q", arguments, forbidden)
		}
	}
	if !reflect.DeepEqual(CompatibilityArguments("auto"), []string{"-profile:v", "high", "-level:v", "4.2"}) {
		t.Fatal("automatic codec lacks H.264 compatibility")
	}
	if CompatibilityArguments("hevc") != nil {
		t.Fatal("HEVC received H.264 compatibility")
	}
}
