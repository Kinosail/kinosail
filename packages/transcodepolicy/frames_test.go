package transcodepolicy

import (
	"strings"
	"testing"
)

func TestCPUFiltersKeepHardwareEncodingWithoutHardwareFrames(t *testing.T) {
	for _, backend := range []string{"qsv", "cuda", "vaapi"} {
		options := Settings{Codec: "h264", Accelerator: backend, Encoder: DefaultEncoder(backend, "h264"), HardwareDecode: true, SoftwareFilters: true, Deinterlace: true, ToneMap: true, CRF: "22"}
		input, output := VideoArguments(options, "1280")
		command := strings.Join(append(input, output...), " ")
		if strings.Contains(command, "-hwaccel ") || !strings.Contains(command, "-c:v "+options.Encoder) || !strings.Contains(command, "bwdif=") || !strings.Contains(command, "tonemap=hable") || !strings.Contains(command, "-color_trc bt709") {
			t.Fatalf("unsafe frame path: %s", command)
		}
		if backend != "cuda" && strings.Index(command, "tonemap=") > strings.Index(command, "hwupload") {
			t.Fatalf("uploaded before CPU filters: %s", command)
		}
	}
}

func TestCudaFramesAndHDRPixelsAreExplicit(t *testing.T) {
	input, output := VideoArguments(Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true}, "640")
	command := strings.Join(append(input, output...), " ")
	if !strings.Contains(command, "-hwaccel_output_format cuda") || !strings.Contains(command, "scale_cuda=") {
		t.Fatalf("CUDA frames were implicit: %s", command)
	}
	for _, hdr := range []string{"hdr10", "hlg"} {
		_, output := VideoArguments(Settings{Codec: "hevc", Accelerator: "none", OutputHDR: hdr}, "640")
		command := strings.Join(output, " ")
		transfer := "smpte2084"
		if hdr == "hlg" {
			transfer = "arib-std-b67"
		}
		if !strings.Contains(command, "format=yuv420p10le") || !strings.Contains(command, "-color_trc "+transfer) || !strings.Contains(command, "-tag:v hvc1") {
			t.Fatalf("HDR pixels and declarations disagree: %s", command)
		}
	}
}
