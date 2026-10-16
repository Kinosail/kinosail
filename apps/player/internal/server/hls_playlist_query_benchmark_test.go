package server

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func BenchmarkHLSPlaylistWithQuery(b *testing.B) {
	var manifest strings.Builder
	manifest.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:2\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	for segment := range 3600 {
		fmt.Fprintf(&manifest, "#EXTINF:2.000000,\nsegment-%05d.m4s\n", segment)
	}
	manifest.WriteString("#EXT-X-ENDLIST\n")
	data := []byte(manifest.String())
	query := url.Values{"playSessionId": {"0123456789abcdef01234567"}, "api_key": {"token"}}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if len(hlsPlaylistWithQuery(data, query)) <= len(data) {
			b.Fatal("query was not applied")
		}
	}
}
