package transcodepolicy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func TestHardwareToneMapFramesPreserveSDRContract(t *testing.T) {
	for _, backend := range []string{"cuda", "qsv", "vaapi", "videotoolbox", "amf", "rkmpp"} {
		for _, method := range ToneMapMethods(backend, "/dev/dri/renderD128") {
			for _, cpu := range []bool{false, true} {
				options := Settings{Codec: "h264", Accelerator: backend, Device: "/dev/dri/renderD128", ToneMap: true, ToneMapInput: "hdr10", HardwareToneMap: method, SoftwareFilters: cpu}
				if backend != "qsv" && backend != "vaapi" {
					options.Device = ""
				}
				input, output := VideoArguments(options, "640")
				command := strings.Join(append(input, output...), " ")
				if !strings.Contains(command, "hwupload") || strings.Contains(command, "tonemap=hable") || !strings.Contains(command, "-color_trc bt709") || !strings.Contains(command, "DOVI_METADATA") {
					t.Fatalf("%s/%s: %s", backend, method, command)
				}
				if ToneMapSoftwareFrames(options) != strings.Contains(command, "hwdownload") {
					t.Fatalf("wrong frame residency: %s", command)
				}
				if cpu && !strings.Contains(command, "scale=w=640") {
					t.Fatalf("missing software subtitle seam: %s", command)
				}
				if method == "opencl" && (backend == "qsv" || backend == "vaapi") && (!strings.Contains(command, "hwmap=derive_device=opencl") || !strings.Contains(command, "-filter_hw_device kino_va")) {
					t.Fatalf("lost selected DRM device: %s", command)
				}
			}
		}
	}
}

func TestInvalidHardwareToneMapCheckHasNoSideEffects(t *testing.T) {
	base := Settings{Codec: "h264", Accelerator: "cuda", Encoder: "h264_nvenc", ToneMap: true, ToneMapInput: "hdr10", HardwareToneMap: "cuda"}
	mutations := []func(*Settings){
		func(s *Settings) { s.ToneMapInput = "" }, func(s *Settings) { s.ToneMapInput = "sdr" }, func(s *Settings) { s.ToneMapInput = "dolby-vision" },
		func(s *Settings) { s.HardwareToneMap = "unknown" }, func(s *Settings) { s.HardwareToneMap = strings.Repeat("x", 1025) },
		func(s *Settings) { s.HardwareToneMap = "qsv" }, func(s *Settings) { s.OutputHDR = "hdr10" }, func(s *Settings) { s.ToneMap = false }, func(s *Settings) { s.HardwareDecode = true },
		func(s *Settings) { s.DisableHardwareToneMap = true },
		func(s *Settings) { s.Accelerator = "qsv"; s.HardwareToneMap = "qsv"; s.ToneMapInput = "hlg" },
	}
	for _, mutate := range mutations {
		options := base
		mutate(&options)
		calls := 0
		result := runCheck(t.Context(), "ffmpeg", options, "GPU", func() (string, error) { calls++; return "", nil }, func(context.Context, string, ...string) error { calls++; return nil })
		if result.Status != "failed" || result.Stage != "setup" || calls != 0 {
			t.Fatalf("invalid settings had effects: %#v calls=%d", options, calls)
		}
	}
}

func TestToneMapCheckRequiresHDRSourceAndSDROutput(t *testing.T) {
	for _, hdr := range []string{"hdr10", "hlg"} {
		for _, outputHDR := range []string{"", "hdr10"} {
			directory := t.TempDir()
			options := Settings{Codec: "h264", Accelerator: "cuda", Encoder: "h264_nvenc", ToneMap: true, ToneMapInput: hdr, HardwareToneMap: "cuda"}
			calls := 0
			result := runCheck(t.Context(), "ffmpeg", options, "GPU", func() (string, error) { return directory, nil }, func(_ context.Context, _ string, args ...string) error {
				calls++
				command := strings.Join(args, " ")
				if calls == 1 {
					transfer := "smpte2084"
					if hdr == "hlg" {
						transfer = "arib-std-b67"
					}
					if !strings.Contains(command, "zscale=pin=bt709") || !strings.Contains(command, "t="+transfer) || !strings.Contains(command, "yuv420p10le") {
						t.Fatalf("not a converted HDR fixture: %s", command)
					}
				}
				if calls >= 4 {
					sample := make([]byte, 32*18)
					for i := range sample {
						sample[i] = byte(32 + i%100)
						if calls == 5 {
							sample[i] += 40
						}
					}
					return os.WriteFile(args[len(args)-1], sample, 0o600)
				}
				if calls == 2 {
					return os.WriteFile(filepath.Join(directory, "init.mp4"), mp4fixture.Initialization(320, 180, "h264", "aac", outputHDR), 0o600)
				}
				return nil
			})
			if outputHDR == "" {
				if result.Status != "passed" || result.HardwareToneMap != "cuda" || result.ToneMapInput != hdr || result.HardwareDecode {
					t.Fatalf("missing specific evidence: %#v", result)
				}
			} else if result.Status != "failed" || result.Stage != "output" || result.HardwareToneMap != "" {
				t.Fatalf("HDR output certified as SDR: %#v", result)
			}
		}
	}
}

func TestToneMapPixelEvidenceRejectsNoOpFlatAndMalformedOutput(t *testing.T) {
	input := make([]byte, 32*18)
	for index := range input {
		input[index] = byte(32 + index%100)
	}
	unchanged := append([]byte(nil), input...)
	noise := append([]byte(nil), input...)
	for index := range noise {
		noise[index]++
	}
	for _, output := range [][]byte{nil, make([]byte, 32*18+1), make([]byte, 32*18), unchanged, noise} {
		if changedToneMapPixels(input, output) {
			t.Fatal("invalid pixel evidence accepted")
		}
	}
	changed := append([]byte(nil), input...)
	for index := range changed {
		changed[index] += 40
	}
	if !changedToneMapPixels(input, changed) {
		t.Fatal("changed luminance rejected")
	}
}
