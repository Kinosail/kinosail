package transcodepolicy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func recordedCheck(t *testing.T, options Settings) (CheckResult, [][]string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "check")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	var arguments [][]string
	result := runCheck(t.Context(), "ffmpeg", options, "GPU", func() (string, error) { return directory, nil }, func(_ context.Context, _ string, values ...string) error {
		arguments = append(arguments, append([]string(nil), values...))
		if len(arguments) == 2 {
			return os.WriteFile(filepath.Join(directory, "init.mp4"), mp4fixture.Initialization(320, 180, options.Codec, "aac", ""), 0o600)
		}
		return nil
	})
	return result, arguments
}

func TestDecodeChecksUseTheRequestedSourceFormat(t *testing.T) {
	for _, test := range []struct {
		input  string
		source []string
	}{
		{"", []string{"-c:v libx264", "-pix_fmt yuv420p"}},
		{"hevc", []string{"-c:v libx265", "-pix_fmt yuv420p"}},
		{"hevc10", []string{"-c:v libx265", "-pix_fmt yuv420p10le"}},
		{"av1", []string{"-c:v libsvtav1", "-pix_fmt yuv420p10le"}},
	} {
		options := Settings{Codec: "h264", Accelerator: "vaapi", Encoder: "h264_vaapi", Device: "/dev/dri/renderD128", HardwareDecode: true, DecodeInput: test.input, Preset: "veryfast", CRF: "22"}
		result, arguments := recordedCheck(t, options)
		if result.Status != "passed" || !result.HardwareDecode || len(arguments) < 2 {
			t.Fatalf("%s check = %#v", test.input, result)
		}
		source, encode := strings.Join(arguments[0], " "), strings.Join(arguments[1], " ")
		for _, expected := range test.source {
			if !strings.Contains(source, expected+" ") {
				t.Errorf("%s source %q lacks %q", test.input, source, expected)
			}
		}
		if !strings.Contains(encode, "-hwaccel vaapi") || strings.Contains(encode, "hevc") || strings.Contains(encode, "av1") {
			t.Errorf("%s encode = %q", test.input, encode)
		}
	}
	if result, arguments := recordedCheck(t, Settings{Codec: "h264", Accelerator: "vaapi", Encoder: "h264_vaapi", DecodeInput: "vp8"}); result.Stage != "setup" || len(arguments) != 0 {
		t.Fatalf("unknown decode input = %#v %v", result, arguments)
	}
}

func TestHardwareFramesToneMapKeepsDecodedFramesOnTheGPU(t *testing.T) {
	for _, backend := range []string{"cuda", "vaapi", "qsv"} {
		options := Settings{Codec: "h264", Accelerator: backend, Encoder: DefaultEncoder(backend, "h264"), Device: "/dev/dri/renderD128", ToneMap: true, ToneMapInput: "hdr10", HardwareToneMap: backend, HardwareDecode: true, DecodeInput: "hevc10", CRF: "22"}
		if backend == "cuda" {
			options.Device = "0"
		}
		input, output := VideoArguments(options, "1280")
		command := strings.Join(append(input, output...), " ")
		for _, expected := range []string{"-hwaccel " + backend, "-hwaccel_output_format " + backend, "DOVI_METADATA", "-color_trc bt709", "w=1280"} {
			if !strings.Contains(command, expected) {
				t.Errorf("%s hardware-frame tone map lacks %q: %s", backend, expected, command)
			}
		}
		if strings.Contains(command, "hwupload") || strings.Contains(command, "hwdownload") || strings.Contains(command, "format=p010le") || !ToneMapHardwareFrames(options) {
			t.Errorf("%s copied frames through system memory: %s", backend, command)
		}
		if result, _ := recordedCheck(t, options); result.Stage == "setup" {
			t.Errorf("%s hardware-frame tone-map check was rejected: %#v", backend, result)
		}
		for _, fallback := range []func(Settings) Settings{
			func(s Settings) Settings { s.DecodeInput = "hevc"; return s },
			func(s Settings) Settings { s.SoftwareFilters = true; return s },
			func(s Settings) Settings { s.HardwareDecode = false; return s },
		} {
			uploaded := fallback(options)
			input, output := VideoArguments(uploaded, "1280")
			if command := strings.Join(append(input, output...), " "); ToneMapHardwareFrames(uploaded) || strings.Contains(command, "-hwaccel ") || !strings.Contains(command, "hwupload") {
				t.Errorf("%s unverified frame path used hardware frames: %s", backend, command)
			}
		}
	}
	opencl := Settings{Codec: "h264", Accelerator: "vaapi", Encoder: "h264_vaapi", ToneMap: true, ToneMapInput: "hdr10", HardwareToneMap: "opencl", HardwareDecode: true, DecodeInput: "hevc10"}
	if result, arguments := recordedCheck(t, opencl); result.Stage != "setup" || len(arguments) != 0 || ToneMapHardwareFrames(opencl) {
		t.Fatalf("unsupported hardware-frame method was checked: %#v", result)
	}
}
