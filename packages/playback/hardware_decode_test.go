package playback

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestSourceTranscodingEnablesHardwareDecodeOnlyForVerifiedInputs(t *testing.T) {
	t.Parallel()
	verified := []string{"h264", "hevc10", "av1"}
	for _, test := range []struct {
		name   string
		video  VideoFacts
		recipe HLSRecipe
		want   string
		decode bool
	}{
		{"eight-bit H.264", VideoFacts{Codec: "h264", BitDepth: 8}, HLSRecipe{Mode: "transcode"}, "h264", true},
		{"ten-bit HEVC", VideoFacts{Codec: "hevc", BitDepth: 10}, HLSRecipe{Mode: "transcode"}, "hevc10", true},
		{"ten-bit HDR HEVC keeps decode for color selection", VideoFacts{Codec: "hevc", BitDepth: 10, HDR: "hdr10"}, HLSRecipe{Mode: "transcode", ToneMap: true}, "hevc10", true},
		{"unverified eight-bit HEVC", VideoFacts{Codec: "hevc", BitDepth: 8}, HLSRecipe{Mode: "transcode"}, "hevc", false},
		{"ten-bit AV1 is distinct from AV1", VideoFacts{Codec: "av1", BitDepth: 10}, HLSRecipe{Mode: "transcode"}, "av110", false},
		{"ten-bit H.264 has no hardware decoder", VideoFacts{Codec: "h264", BitDepth: 10}, HLSRecipe{Mode: "transcode"}, "", false},
		{"interlaced", VideoFacts{Codec: "h264", BitDepth: 8, FieldOrder: "tt"}, HLSRecipe{Mode: "transcode"}, "h264", false},
		{"burned subtitles", VideoFacts{Codec: "hevc", BitDepth: 10}, HLSRecipe{Mode: "transcode", Burn: "text"}, "hevc10", false},
		{"anamorphic", VideoFacts{Codec: "h264", BitDepth: 8, SampleAspectRatio: "16:15"}, HLSRecipe{Mode: "transcode"}, "h264", false},
	} {
		options := SourceTranscoding(transcodepolicy.Settings{Accelerator: "cuda", HardwareDecodeInputs: verified}, MediaFacts{Video: test.video}, test.recipe)
		if options.DecodeInput != test.want || options.HardwareDecode != test.decode {
			t.Errorf("%s: input %q decode %v (%#v)", test.name, options.DecodeInput, options.HardwareDecode, options)
		}
	}
	software := SourceTranscoding(transcodepolicy.Settings{Accelerator: "none"}, MediaFacts{Video: VideoFacts{Codec: "hevc", BitDepth: 10}}, HLSRecipe{Mode: "transcode"})
	if software.HardwareDecode || software.DecodeInput != "hevc10" {
		t.Fatalf("software settings advertised hardware decode: %#v", software)
	}
}
