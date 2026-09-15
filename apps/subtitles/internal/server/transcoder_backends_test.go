package server

import (
	"strings"
	"testing"
)

func TestEveryHardwareBackendHasAnFFmpegRecipe(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"qsv":          {"-hwaccel qsv", "scale_qsv", "-c:v h264_qsv"},
		"cuda":         {"-hwaccel cuda", "scale_cuda", "-c:v h264_nvenc"},
		"vaapi":        {"-hwaccel vaapi", "scale_vaapi", "-c:v h264_vaapi"},
		"rkmpp":        {"-init_hw_device rkmpp=rk", "-hwaccel rkmpp", "-hwaccel_output_format drm_prime", "scale_rkrga", "-c:v h264_rkmpp", "-rc_mode CQP", "-qp_init 22"},
		"v4l2m2m":      {"scale=w=", "-c:v h264_v4l2m2m"},
		"videotoolbox": {"-hwaccel videotoolbox", "-c:v h264_videotoolbox"},
		"amf":          {"-hwaccel d3d11va", "-hwaccel_output_format d3d11", "scale_d3d11", "-c:v h264_amf"},
		"mf":           {"-hwaccel d3d11va", "-hwaccel_output_format d3d11", "scale_d3d11", "-c:v h264_mf", "-hw_encoding 1"},
	}
	for accelerator, expected := range tests {
		input, output := videoArguments(transcodeSettings{Accelerator: accelerator, CRF: "22", Preset: "veryfast"}, "1280")
		arguments := strings.Join(append(input, output...), " ")
		for _, value := range expected {
			if !strings.Contains(arguments, value) {
				t.Errorf("%s arguments %q lack %q", accelerator, arguments, value)
			}
		}
	}
}

func TestRKMPPToneMappingUsesSoftwareDecodeAndFilterBeforeHardwareEncode(t *testing.T) {
	t.Parallel()
	input, output := videoArguments(transcodeSettings{Accelerator: "rkmpp", CRF: "22", Preset: "veryfast", ToneMap: true}, "1280")
	arguments := strings.Join(append(input, output...), " ")
	for _, expected := range []string{"zscale=t=linear", "tonemap=hable", "format=yuv420p", "scale=w=", "-c:v h264_rkmpp", "-rc_mode CQP", "-qp_init 22"} {
		if !strings.Contains(arguments, expected) {
			t.Errorf("RKMPP tone-map arguments %q lack %q", arguments, expected)
		}
	}
	for _, hardwareInput := range []string{"-hwaccel rkmpp", "drm_prime", "scale_rkrga"} {
		if strings.Contains(arguments, hardwareInput) {
			t.Errorf("RKMPP tone-map arguments %q unexpectedly use %q", arguments, hardwareInput)
		}
	}
}

func TestIntelBackendsUseTheDetectedRenderDevice(t *testing.T) {
	t.Parallel()
	const device = "/dev/dri/renderD129"
	for _, accelerator := range []string{"qsv", "vaapi"} {
		input, _ := videoArguments(transcodeSettings{Accelerator: accelerator, Device: device}, "1280")
		if arguments := strings.Join(input, " "); !strings.Contains(arguments, device) {
			t.Errorf("%s input arguments %q lack detected device %q", accelerator, arguments, device)
		}
	}
}
