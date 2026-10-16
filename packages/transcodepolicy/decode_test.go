package transcodepolicy

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDecodeInputsCoverTenBitAndModernCodecs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		codec string
		depth int
		want  string
	}{
		{"h264", 8, "h264"}, {"h264", 0, "h264"}, {"h264", 10, ""}, {"hevc", 8, "hevc"}, {"hevc", 10, "hevc10"}, {"hevc", 12, ""},
		{"vp9", 8, "vp9"}, {"vp9", 10, "vp910"}, {"av1", 8, "av1"}, {"av1", 10, "av110"}, {"mpeg2video", 8, ""}, {"", 8, ""},
	} {
		if got := DecodeInput(test.codec, test.depth); got != test.want {
			t.Errorf("DecodeInput(%q, %d) = %q, want %q", test.codec, test.depth, got, test.want)
		}
	}
	for _, input := range DecodeInputs() {
		codec, depth := DecodeInputCodec(input)
		if !ValidDecodeInput(input) || DecodeInput(codec, depth) != input {
			t.Errorf("decode input %q does not round trip (%q, %d)", input, codec, depth)
		}
	}
	if ValidDecodeInput("h26410") || ValidDecodeInput("") {
		t.Fatal("unknown decode input accepted")
	}
	options := Settings{DecodeInput: "hevc10", HardwareDecodeInputs: []string{"h264", "hevc10"}}
	if !HardwareDecodeAvailable(options) || HardwareDecodeAvailable(Settings{DecodeInput: "av1", HardwareDecodeInputs: options.HardwareDecodeInputs}) || HardwareDecodeAvailable(Settings{HardwareDecodeInputs: []string{""}}) {
		t.Fatal("verified decode inputs were not honored")
	}
}

func TestHardwareDecodedToneMapKeepsFramesOnTheDevice(t *testing.T) {
	t.Parallel()
	for _, backend := range []string{"cuda", "vaapi", "qsv"} {
		options := Settings{Codec: "h264", Accelerator: backend, Device: "/dev/dri/renderD128", ToneMap: true, ToneMapInput: "hdr10", HardwareToneMap: backend, HardwareDecode: true, DecodeInput: "hevc10"}
		if backend == "cuda" {
			options.Device = "1"
		}
		if !HardwareDecodedToneMap(options) {
			t.Fatalf("%s: hardware decoded tone map rejected", backend)
		}
		input, output := VideoArguments(options, "1280")
		command := strings.Join(append(input, output...), " ")
		if !strings.Contains(command, "-hwaccel "+backend) || strings.Contains(command, "hwupload") || strings.Contains(command, "hwdownload") || !strings.Contains(command, "DOVI_METADATA") || !strings.Contains(command, "-color_trc bt709") {
			t.Fatalf("%s: frames left the device: %s", backend, command)
		}
		if backend == "cuda" && (!strings.Contains(command, "-hwaccel_device 1") || !strings.Contains(command, "-vf tonemap_cuda=tonemap=bt2390") || !strings.Contains(command, "scale_cuda=w=1280:h=-2:format=nv12")) {
			t.Fatalf("cuda tone map lost its device or scaler: %s", command)
		}
		if backend != "cuda" && !strings.Contains(command, "/dev/dri/renderD128") {
			t.Fatalf("%s tone map lost its render device: %s", backend, command)
		}
		for _, change := range []func(*Settings){
			func(s *Settings) { s.SoftwareFilters = true }, func(s *Settings) { s.HardwareToneMap = "opencl" }, func(s *Settings) { s.HardwareDecode = false },
		} {
			variant := options
			change(&variant)
			if HardwareDecodedToneMap(variant) {
				t.Fatalf("%s: software frames or filters still claimed hardware decode: %#v", backend, variant)
			}
			input, output := VideoArguments(variant, "1280")
			if command := strings.Join(append(input, output...), " "); variant.HardwareToneMap != "opencl" && !strings.Contains(command, "format=p010le,hwupload") {
				t.Fatalf("%s: software decode lost its upload: %s", backend, command)
			}
		}
	}
}

func TestHardwareDecodedHDROutputUsesTenBitDeviceSurfaces(t *testing.T) {
	t.Parallel()
	for backend, want := range map[string]string{"cuda": "scale_cuda=w=1280:h=-2:format=p010le", "vaapi": "scale_vaapi=w=1280:h=-2:format=p010le", "qsv": "scale_qsv=w=1280:h=-2:format=p010le", "rkmpp": "scale_rkrga=w=1280:h=-2:format=p010le"} {
		input, output := VideoArguments(Settings{Codec: "hevc", Accelerator: backend, Encoder: DefaultEncoder(backend, "hevc"), OutputHDR: "hdr10", HardwareDecode: true, DecodeInput: "hevc10"}, "1280")
		command := strings.Join(append(input, output...), " ")
		if !strings.Contains(command, "-hwaccel "+backend) || !strings.Contains(command, want) || !strings.Contains(command, "-color_trc smpte2084") || strings.Contains(command, "yuv420p10le") {
			t.Fatalf("%s HDR frames: %s", backend, command)
		}
	}
	_, output := VideoArguments(Settings{Codec: "hevc", Accelerator: "cuda", OutputHDR: "hlg", HardwareDecode: true}, "640")
	if command := strings.Join(output, " "); !strings.Contains(command, "format=p010le") || !strings.Contains(command, "-color_trc arib-std-b67") {
		t.Fatalf("HLG CUDA output: %s", command)
	}
}

func TestEncoderPresetsFollowTheConversionPreference(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ accelerator, preset, crf, want string }{
		{"cuda", "ultrafast", "26", "-preset p2"}, {"cuda", "veryfast", "22", "-preset p4"}, {"cuda", "medium", "19", "-preset p6"},
		{"qsv", "ultrafast", "26", "-preset veryfast"}, {"qsv", "medium", "19", "-preset medium"},
		{"videotoolbox", "ultrafast", "26", "-q:v 55"}, {"videotoolbox", "medium", "19", "-q:v 75"}, {"videotoolbox", "veryfast", "unknown", "-q:v 65"},
	} {
		_, output := VideoArguments(Settings{Codec: "h264", Accelerator: test.accelerator, Preset: test.preset, CRF: test.crf}, "1280")
		if command := strings.Join(output, " "); !strings.Contains(command, test.want) {
			t.Errorf("%s/%s: %s lacks %s", test.accelerator, test.preset, command, test.want)
		}
	}
}

func TestDecodeCheckSetupRejectsUnverifiableCombinations(t *testing.T) {
	t.Parallel()
	base := Settings{Codec: "h264", Accelerator: "cuda", Encoder: "h264_nvenc", HardwareDecode: true, DecodeInput: "hevc10"}
	if message := checkSetup("ffmpeg", base); message != "" {
		t.Fatalf("valid decode check rejected: %s", message)
	}
	for name, change := range map[string]func(*Settings){
		"unknown input":    func(s *Settings) { s.DecodeInput = "mpeg2" },
		"missing input":    func(s *Settings) { s.DecodeInput = "" },
		"software decoder": func(s *Settings) { s.Accelerator = "amf"; s.Encoder = "h264_amf" },
		"eight bit tone map": func(s *Settings) {
			s.ToneMap, s.ToneMapInput, s.HardwareToneMap, s.DecodeInput = true, "hdr10", "cuda", "hevc"
		},
		"eight bit HDR": func(s *Settings) {
			s.Codec, s.Encoder, s.OutputHDR, s.DecodeInput = "hevc", "hevc_nvenc", "hdr10", "av1"
		},
		"software tone frames": func(s *Settings) {
			s.ToneMap, s.ToneMapInput, s.HardwareToneMap, s.SoftwareFilters = true, "hdr10", "cuda", true
		},
	} {
		options := base
		change(&options)
		if message := checkSetup("ffmpeg", options); message == "" {
			t.Errorf("%s: unverifiable decode check accepted: %#v", name, options)
		}
	}
	software := Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264", DecodeInput: "hevc10"}
	if message := checkSetup("ffmpeg", software); message != "" {
		t.Fatalf("software decode of a labelled input rejected: %s", message)
	}
	toneMap := Settings{Codec: "h264", Accelerator: "vaapi", Encoder: "h264_vaapi", ToneMap: true, ToneMapInput: "hlg", HardwareToneMap: "vaapi", HardwareDecode: true, DecodeInput: "hevc10"}
	if message := checkSetup("ffmpeg", toneMap); message == "" {
		t.Fatal("VA-API HLG tone mapping accepted")
	}
}

func TestDecodeCheckSourcesMatchTheRequestedInput(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		options       Settings
		wantSource    string
		wantEncoders  []string
		wantFragments []string
	}{
		{Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true, DecodeInput: "hevc10"}, "source.mkv", []string{"libx265"}, []string{"-pix_fmt yuv420p10le", "-preset ultrafast"}},
		{Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true, DecodeInput: "av110"}, "source.mkv", []string{"libsvtav1", "libaom-av1", "librav1e"}, []string{"-pix_fmt yuv420p10le"}},
		{Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true, DecodeInput: "vp910"}, "source.mkv", []string{"libvpx-vp9"}, []string{"-profile:v 2", "-pix_fmt yuv420p10le"}},
		{Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true, DecodeInput: "vp9"}, "source.mkv", []string{"libvpx-vp9"}, []string{"-pix_fmt yuv420p"}},
		{Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true, DecodeInput: "hevc10", ToneMap: true, ToneMapInput: "hlg", HardwareToneMap: "cuda"}, "source.mkv", []string{"libx265"}, []string{"t=arib-std-b67", "-color_trc arib-std-b67", "-pix_fmt yuv420p10le"}},
		{Settings{Codec: "hevc", Accelerator: "vaapi", HardwareDecode: true, DecodeInput: "hevc10", OutputHDR: "hdr10"}, "source.mkv", []string{"libx265"}, []string{"t=smpte2084", "-color_trc smpte2084"}},
		{Settings{Codec: "hevc", Accelerator: "vaapi", OutputHDR: "hdr10"}, "source.mp4", []string{"libx264"}, []string{"-pix_fmt yuv420p"}},
		{Settings{Codec: "h264", Accelerator: "cuda", ToneMap: true, ToneMapInput: "hdr10", HardwareToneMap: "cuda"}, "source.mkv", []string{"ffv1"}, []string{"t=smpte2084"}},
		{Settings{Codec: "h264", Accelerator: "none", DecodeInput: "hevc10"}, "source.mp4", []string{"libx264"}, nil},
	} {
		source, candidates := checkSources("/work", test.options)
		if filepath.Base(source) != test.wantSource || len(candidates) != len(test.wantEncoders) {
			t.Fatalf("%#v: source %q candidates %v", test.options, source, candidates)
		}
		for index, arguments := range candidates {
			command := strings.Join(arguments, " ")
			if !strings.Contains(command, "-c:v "+test.wantEncoders[index]) || !strings.HasSuffix(command, "-c:a aac "+source) || !strings.Contains(command, "testsrc2=size=640x360:rate=24:duration=1") {
				t.Fatalf("%s candidate %d: %s", test.options.DecodeInput, index, command)
			}
			for _, fragment := range test.wantFragments {
				if !strings.Contains(command, fragment) {
					t.Fatalf("%s candidate %d lacks %q: %s", test.options.DecodeInput, index, fragment, command)
				}
			}
		}
	}
}

func TestRealFFmpegProducesEveryDecodeCheckSource(t *testing.T) {
	t.Parallel()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	for _, input := range DecodeInputs() {
		for _, hdr := range []string{"", "hdr10"} {
			options := Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true, DecodeInput: input}
			if hdr != "" {
				if _, depth := DecodeInputCodec(input); depth != 10 {
					continue
				}
				options.ToneMap, options.ToneMapInput, options.HardwareToneMap = true, hdr, "cuda"
			}
			directory := t.TempDir()
			source, candidates := checkSources(directory, options)
			if err := runFirst(t.Context(), ffmpeg, candidates, runFFmpeg); err != nil {
				t.Fatalf("%s/%s: no candidate produced a source: %v", input, hdr, err)
			}
			probe, err := exec.CommandContext(context.Background(), ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name,pix_fmt,color_transfer", "-of", "csv=p=0", source).Output()
			if err != nil {
				t.Fatalf("%s/%s: probe failed: %v", input, hdr, err)
			}
			fields := strings.Split(strings.TrimSpace(string(probe)), ",")
			codec, depth := DecodeInputCodec(input)
			wantFormat := "yuv420p"
			if depth == 10 {
				wantFormat = "yuv420p10le"
			}
			if len(fields) < 2 || fields[0] != codec || fields[1] != wantFormat || hdr != "" && !slices.Contains(fields, "smpte2084") {
				t.Fatalf("%s/%s: source stream = %q", input, hdr, probe)
			}
		}
	}
}
