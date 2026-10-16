package playback

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestHardwareDecodeFollowsVerifiedSourceFormats(t *testing.T) {
	verified := transcodepolicy.Settings{Codec: "h264", Accelerator: "vaapi", HardwareDecode: true, HardwareDecodeHEVC10: true}
	for _, test := range []struct {
		name   string
		video  VideoFacts
		recipe HLSRecipe
		input  string
		decode bool
	}{
		{"8-bit H.264", VideoFacts{Codec: "h264", BitDepth: 8}, HLSRecipe{Mode: "transcode"}, "h264", true},
		{"8-bit HEVC", VideoFacts{Codec: "hevc", BitDepth: 8}, HLSRecipe{Mode: "transcode"}, "hevc", false},
		{"10-bit HEVC", VideoFacts{Codec: "hevc", BitDepth: 10}, HLSRecipe{Mode: "transcode"}, "hevc10", true},
		{"AV1", VideoFacts{Codec: "av1", BitDepth: 10}, HLSRecipe{Mode: "transcode"}, "av1", false},
		{"12-bit HEVC", VideoFacts{Codec: "hevc", BitDepth: 12}, HLSRecipe{Mode: "transcode"}, "", false},
		{"10-bit H.264", VideoFacts{Codec: "h264", BitDepth: 10}, HLSRecipe{Mode: "transcode"}, "", false},
		{"rotated HEVC", VideoFacts{Codec: "hevc", BitDepth: 10, Rotation: 90}, HLSRecipe{Mode: "transcode"}, "hevc10", false},
		{"HDR HEVC tone map", VideoFacts{Codec: "hevc", BitDepth: 10, HDR: "hdr10"}, HLSRecipe{Mode: "transcode", ToneMap: true}, "hevc10", false},
	} {
		options := SourceTranscoding(verified, MediaFacts{Video: test.video}, test.recipe)
		if options.DecodeInput != test.input || options.HardwareDecode != test.decode {
			t.Errorf("%s: input=%q decode=%t, want %q %t", test.name, options.DecodeInput, options.HardwareDecode, test.input, test.decode)
		}
		if !options.HardwareDecodeHEVC10 || options.HardwareDecodeHEVC || options.HardwareDecodeAV1 {
			t.Errorf("%s changed verified decode evidence: %#v", test.name, options)
		}
	}
}
