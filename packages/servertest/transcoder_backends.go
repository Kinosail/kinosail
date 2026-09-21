package servertest

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func EveryHardwareBackendHasAnFFmpegRecipe(t *testing.T, videoArguments func(transcodepolicy.Settings, string) ([]string, []string)) {
	t.Parallel()
	tests := map[string][]string{
		"qsv":          {"-init_hw_device qsv=kino", "hwupload", "-c:v h264_qsv"},
		"cuda":         {"scale=w=", "-c:v h264_nvenc"},
		"vaapi":        {"-init_hw_device vaapi=kino:", "hwupload", "-c:v h264_vaapi"},
		"rkmpp":        {"scale=w=", "-c:v h264_rkmpp", "-rc_mode CQP", "-qp_init 22"},
		"v4l2m2m":      {"scale=w=", "-c:v h264_v4l2m2m"},
		"videotoolbox": {"scale=w=", "-c:v h264_videotoolbox"},
		"amf":          {"scale=w=", "-c:v h264_amf"},
		"mf":           {"scale=w=", "-c:v h264_mf", "-hw_encoding 1"},
	}
	for accelerator, expected := range tests {
		input, output := videoArguments(transcodepolicy.Settings{Accelerator: accelerator, CRF: "22", Preset: "veryfast"}, "1280")
		arguments := strings.Join(append(input, output...), " ")
		if strings.Contains(arguments, "-hwaccel ") {
			t.Errorf("%s enabled unrequested hardware decoding", accelerator)
		}
		for _, value := range expected {
			if !strings.Contains(arguments, value) {
				t.Errorf("%s arguments %q lack %q", accelerator, arguments, value)
			}
		}
	}
}

func RKMPPToneMappingUsesSoftwareDecodeAndFilterBeforeHardwareEncode(t *testing.T, videoArguments func(transcodepolicy.Settings, string) ([]string, []string)) {
	t.Parallel()
	input, output := videoArguments(transcodepolicy.Settings{Accelerator: "rkmpp", CRF: "22", Preset: "veryfast", ToneMap: true}, "1280")
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

func IntelBackendsUseTheDetectedRenderDevice(t *testing.T, videoArguments func(transcodepolicy.Settings, string) ([]string, []string)) {
	t.Parallel()
	const device = "/dev/dri/renderD129"
	for _, accelerator := range []string{"qsv", "vaapi"} {
		input, _ := videoArguments(transcodepolicy.Settings{Accelerator: accelerator, Device: device}, "1280")
		if arguments := strings.Join(input, " "); !strings.Contains(arguments, device) {
			t.Errorf("%s input arguments %q lack detected device %q", accelerator, arguments, device)
		}
	}
}
