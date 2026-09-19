package playback

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestSourceToneMapUsesBaseLayerAndKeepsCPUFilters(t *testing.T) {
	for _, test := range []struct {
		hdr           string
		compatibility int
		want          string
	}{
		{"hdr10", 0, "hdr10"}, {"hdr10+", 0, "hdr10"}, {"hlg", 0, "hlg"}, {"dolby-vision", 1, "hdr10"}, {"dolby-vision", 4, "hlg"},
	} {
		facts := MediaFacts{Video: VideoFacts{Codec: "hevc", HDR: test.hdr, DolbyVisionCompatibility: test.compatibility, BitDepth: 10}}
		options := SourceTranscoding(transcodepolicy.Settings{HardwareDecode: true}, facts, HLSRecipe{Mode: "transcode", ToneMap: true, Burn: "text"})
		if options.ToneMapInput != test.want || !options.SoftwareFilters || options.HardwareDecode || !options.ToneMap {
			t.Fatalf("source settings: %#v", options)
		}
	}
}

func TestHardwareToneMappingPrecedesSubtitleRendering(t *testing.T) {
	options := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", ToneMap: true, ToneMapInput: "hdr10", HardwareToneMap: "cuda", SoftwareFilters: true}
	_, video := transcodepolicy.VideoArguments(options, "640")
	mapping, _ := ApplyBurnIn(video, "/library/movie.mkv", HLSRecipe{Mode: "transcode", Burn: "text", Subtitle: 1}, HLSRecipePolicy{})
	graph := strings.Join(mapping, " ")
	download, burn, scale := strings.Index(graph, "hwdownload"), strings.Index(graph, "subtitles="), strings.Index(graph, "scale=w=")
	if download < 0 || burn < download || scale < burn {
		t.Fatalf("SDR subtitle colors or CPU frames lost: %s", graph)
	}
}

func TestSeekCannotMixToneMappingImplementations(t *testing.T) {
	dir := t.TempDir()
	options := transcodepolicy.Settings{Accelerator: "cuda", Encoder: "h264_nvenc", Device: "0", Codec: "h264", ToneMap: true, HardwareToneMap: "cuda"}
	if err := BindHLSEncoder(dir, options, false); err != nil {
		t.Fatal(err)
	}
	if err := BindHLSEncoder(dir, options, true); err != nil {
		t.Fatal(err)
	}
	options.HardwareToneMap = ""
	if err := BindHLSEncoder(dir, options, true); err == nil {
		t.Fatal("seek mixed GPU and software tone mapping")
	}
}
