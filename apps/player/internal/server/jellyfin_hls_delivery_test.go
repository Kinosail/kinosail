package server_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func assertRestartedJellyfinHLSChain(t *testing.T, config server.Config, masterPath string) {
	t.Helper()
	masterURL, err := url.Parse(masterPath)
	if err != nil {
		t.Fatal(err)
	}
	restarted := newJellyfinServer(t, config)
	master := jellyfinCall(t, restarted, http.MethodGet, masterPath, "", "")
	childMatch := regexp.MustCompile(`(?m)^([0-9]+p/index\.m3u8\?[^\r\n]+)$`).FindStringSubmatch(master.Body.String())
	if master.Code != http.StatusOK || len(childMatch) != 2 {
		t.Fatalf("transcode master after restart = %d %q", master.Code, master.Body.String())
	}
	childReference, err := url.Parse(childMatch[1])
	if err != nil {
		t.Fatal(err)
	}
	childURL := masterURL.ResolveReference(childReference)
	variant := jellyfinCall(t, restarted, http.MethodGet, childURL.String(), "", "")
	segmentMatch := regexp.MustCompile(`(?m)^(segment-00000\.m4s\?[^\r\n]+)$`).FindStringSubmatch(variant.Body.String())
	if variant.Code != http.StatusOK || len(segmentMatch) != 2 {
		t.Fatalf("transcode child after restart = %d %q", variant.Code, variant.Body.String())
	}
	segmentReference, err := url.Parse(segmentMatch[1])
	if err != nil {
		t.Fatal(err)
	}
	segment := jellyfinCall(t, restarted, http.MethodGet, childURL.ResolveReference(segmentReference).String(), "", "")
	if segment.Code != http.StatusOK || segment.Header().Get("Content-Type") != "video/iso.segment" {
		t.Fatalf("transcode segment after restart = %d %q", segment.Code, segment.Body.String())
	}
}

func assertJellyfinSessionHLSChain(t *testing.T, handler http.Handler, id, playID string) {
	t.Helper()
	assertJellyfinHLSChain(t, handler, "/Videos/"+id+"/stream?playSessionId="+playID, "/Videos/"+id+"/", playID)
}

func assertJellyfinSourceHLSChain(t *testing.T, handler http.Handler, id, playID string) {
	t.Helper()
	base := "/Videos/" + id + "/source/hls1/main/"
	assertJellyfinHLSChain(t, handler, base+"index.m3u8?playSessionId="+playID, base, playID)
}

func assertJellyfinHLSChain(t *testing.T, handler http.Handler, masterURL, childBase, playID string) {
	t.Helper()
	master := jellyfinCall(t, handler, http.MethodGet, masterURL, "", "")
	if master.Code != http.StatusOK || !strings.Contains(master.Body.String(), "#EXTM3U") {
		t.Fatalf("session-plan HLS delivery = %d %q", master.Code, master.Body.String())
	}
	child := regexp.MustCompile(`(?m)^([0-9]+p/index\.m3u8)\?`).FindStringSubmatch(master.Body.String())
	if len(child) != 2 {
		t.Fatalf("session-plan HLS child = %q", master.Body.String())
	}
	variant := jellyfinCall(t, handler, http.MethodGet, childBase+child[1]+"?playSessionId="+playID, "", "")
	if variant.Code != http.StatusOK || !strings.Contains(variant.Body.String(), "#EXTINF") || strings.Contains(variant.Body.String(), "#EXT-X-STREAM-INF") {
		t.Fatalf("session-plan HLS variant = %d %q", variant.Code, variant.Body.String())
	}
	quality := strings.TrimSuffix(child[1], "/index.m3u8")
	assertJellyfinHLSAssets(t, handler, childBase, quality, playID)
}

func assertJellyfinHLSAssets(t *testing.T, handler http.Handler, childBase, quality, playID string) {
	t.Helper()
	for asset, contentType := range map[string]string{"init.mp4": "video/mp4", "segment-00000.m4s": "video/iso.segment"} {
		response := jellyfinCall(t, handler, http.MethodGet, childBase+quality+"/"+asset+"?playSessionId="+playID, "", "")
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != contentType || response.Body.Len() == 0 {
			t.Fatalf("session-plan HLS asset %s = %d %q %q", asset, response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
	}
}
