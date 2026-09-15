package servertest

import (
	"os"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func SingleSegmentPlayableHLS() string {
	return `for output; do
case "$output" in
  */index.m3u8) directory=${output%/*}; mkdir -p "$directory"; ` + mp4fixture.Shell(mp4fixture.Initialization(640, 360, "h264", "aac", "")) + ` > "$directory/init.mp4"; printf segment > "$directory/segment-00000.m4s"; printf '#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI="init.mp4"\n#EXTINF:4,\nsegment-00000.m4s\n#EXT-X-ENDLIST\n' > "$output" ;;
esac
done
`
}

func WriteExecutable(t *testing.T, path, content string) {
	t.Helper()
	//nolint:gosec // G306: executable test fixtures are intentionally owner-only.
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func PlayableHLS() string { return PlayableHLSWithMedia(640, 360, "aac") }

func PlayableHLSWithMedia(width, height int, audioCodec string) string {
	return PlayableHLSWithFormat(width, height, "h264", audioCodec)
}

func PlayableHLSWithFormat(width, height int, codec, audioCodec string) string {
	return playableHLS(mp4fixture.Shell(mp4fixture.Initialization(width, height, codec, audioCodec, "")))
}

func playableHLS(initialization string) string {
	return `for output; do
case "$output" in
  */index.m3u8) directory=${output%/*}; mkdir -p "$directory"; ` + initialization + ` > "$directory/init.mp4"; printf segment > "$directory/segment-00000.m4s"; printf segment > "$directory/segment-00001.m4s"; printf '#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI="init.mp4"\n#EXTINF:4,\nsegment-00000.m4s\n#EXTINF:4,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n' > "$output" ;;
esac
done
`
}

// HardwareSmokeHLS supplies format-correct synthetic sample descriptions for
// mocked operation checks. It does not certify a real encoder or decoder.
func HardwareSmokeHLS() string {
	var result strings.Builder
	result.WriteString("case \"$*\" in *kinosail-transcoder-check-*)\ncase \"$*\" in\n")
	for _, format := range []struct{ pattern, codec, hdr string }{
		{"*hevc*smpte2084*|*libx265*smpte2084*", "hevc", "hdr10"},
		{"*hevc*arib-std-b67*|*libx265*arib-std-b67*", "hevc", "hlg"},
		{"*hevc*|*libx265*", "hevc", ""},
		{"*av1*", "av1", ""},
		{"*vp9*", "vp9", ""},
		{"*", "h264", ""},
	} {
		result.WriteString(format.pattern + ")\n" + playableHLS(mp4fixture.Shell(mp4fixture.Initialization(320, 180, format.codec, "aac", format.hdr))) + ";;\n")
	}
	result.WriteString("esac\nexit 0;;\nesac\n")
	return result.String()
}
