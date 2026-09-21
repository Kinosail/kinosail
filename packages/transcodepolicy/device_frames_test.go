package transcodepolicy

import (
	"slices"
	"strings"
	"testing"
)

func TestCudaDeviceSelectionAppliesToDecodeAndEncode(t *testing.T) {
	input, output := VideoArguments(Settings{Codec: "h264", Accelerator: "cuda", Device: "2", HardwareDecode: true, CRF: "22"}, "640")
	if !strings.Contains(strings.Join(input, " "), "-hwaccel_device 2") || !strings.Contains(strings.Join(output, " "), "-gpu 2") {
		t.Fatalf("device lost: %v / %v", input, output)
	}
}

func TestHardwareHDRUsesTenBitSurfaces(t *testing.T) {
	for _, backend := range []string{"qsv", "vaapi"} {
		input, output := VideoArguments(Settings{Codec: "hevc", Accelerator: backend, OutputHDR: "hdr10", HardwareDecode: true}, "1280")
		command := strings.Join(append(input, output...), " ")
		if !strings.Contains(command, "format=p010le") || !strings.Contains(command, "-color_trc smpte2084") {
			t.Fatalf("HDR surface lost: %s", command)
		}
	}
}

func TestToneMapMethodsDistinguishWindowsQSVFromDRM(t *testing.T) {
	if got := ToneMapMethods("qsv", "0"); !slices.Equal(got, []string{"qsv", "d3d11"}) {
		t.Fatalf("Windows methods = %v", got)
	}
	if got := ToneMapMethods("unknown", ""); got != nil {
		t.Fatalf("unknown methods = %v", got)
	}
}
