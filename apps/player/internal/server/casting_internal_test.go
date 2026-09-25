package server

import (
	"crypto/sha256"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCastRegistryExpiresAndChecksOwnership(t *testing.T) {
	now := time.Now()
	service := &castService{sessions: make(map[string]castSession), now: func() time.Time { return now }}
	id := strings.Repeat("a", 32)
	service.sessions[id] = castSession{ID: id, profileID: "viewer", ExpiresAt: now.Add(time.Hour), proof: sha256.Sum256([]byte("secret"))}
	service.remove(id, "other")
	if _, ok := service.session(id); !ok {
		t.Fatal("other viewer revoked session")
	}
	now = now.Add(2 * time.Hour)
	if _, ok := service.session(id); ok || len(service.sessions) != 0 {
		t.Fatal("expired grant survived")
	}
}

func TestCastHLSKeepsScopedTicketOnAllChildResources(t *testing.T) {
	manifest := []byte("#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\nsegment-1.m4s\nvariant/index.m3u8\n")
	result := string(hlsPlaylistWithQuery(manifest, url.Values{"ticket": {"scoped"}}))
	for _, expected := range []string{`URI="init.mp4?ticket=scoped"`, "segment-1.m4s?ticket=scoped", "variant/index.m3u8?ticket=scoped"} {
		if !strings.Contains(result, expected) {
			t.Errorf("missing %s", expected)
		}
	}
	if strings.Contains(result, "api_key") || strings.Contains(result, "Authorization") {
		t.Fatal("account credentials leaked into playlist")
	}
}

func TestTVPickerExposesProtocolsAndLargerEntryForVideoAndAudio(t *testing.T) {
	input := `<video data-title="{{.Title}}"></video><button class="quiet" type="button" aria-label="Play on device" data-cast>{{icon "cast"}}</button><audio data-title="{{.Title}}"></audio></main>`
	result := tvPlayerTemplate(input)
	if strings.Count(result, `data-tv-open`) != 2 || strings.Count(result, `data-cast-api=`) != 2 {
		t.Fatal("video or audio lacks TV entry point")
	}
	for _, label := range []string{"Play on another device", "HomePod", "Google Cast / Chromecast", "Cast speakers", "DLNA / UPnP receivers", "Find DLNA receivers", "Screen mirroring / Miracast", "Stop casting"} {
		if !strings.Contains(result, label) {
			t.Errorf("missing protocol or control %q", label)
		}
	}
}
