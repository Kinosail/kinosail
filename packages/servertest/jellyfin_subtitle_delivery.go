package servertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

type jellyfinSubtitleSource struct {
	SupportsDirectPlay      bool
	TranscodingURL          string `json:"TranscodingUrl"`
	DefaultAudioStreamIndex int
	MediaStreams            []struct {
		Index       int
		Type        string
		Codec       string
		IsExternal  bool
		DeliveryURL string `json:"DeliveryUrl"`
	}
}

func (fixture EmbeddedSubtitleFixture) JellyfinSubtitleChoicesUseConvertedTextInsteadOfBurnIn(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler := fixture.jellyfinSubtitleServer(t)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := fixture.Login(t, handler, owner)
	var catalog struct{ Items []struct{ ID string } }
	fixture.Decode(t, fixture.Call(t, handler, http.MethodGet, "/Items", "", token), &catalog)
	itemID := catalog.Items[0].ID
	webvtt := fixture.jellyfinSubtitleSource(t, handler, token, itemID, `[{"Format":"vtt","Method":"External"}]`, 3, 2)
	assertJellyfinDirectSubtitle(t, webvtt, 2)
	assertUniqueJellyfinStreamIndexes(t, webvtt)
	assertJellyfinSubtitleBody(t, handler, jellyfinDelivery(t, webvtt, 3), "/Stream.vtt", "WEBVTT", "00:00.000 --> 00:01.000\nDoor closes")
	sidecar := jellyfinExternalIndex(t, webvtt)
	external := fixture.jellyfinSubtitleSource(t, handler, token, itemID, `[{"Format":"vtt","Method":"External"}]`, sidecar, 1)
	assertJellyfinDirectSubtitle(t, external, 1)
	assertJellyfinSubtitleBody(t, handler, jellyfinDelivery(t, external, sidecar), "/Stream.vtt", "WEBVTT", "Café crème")
	captions := fixture.jellyfinSubtitleSource(t, handler, token, itemID, `[{"Format":"vtt","Method":"External"}]`, 5, 1)
	assertJellyfinDirectSubtitle(t, captions, 1)
	assertJellyfinSubtitleBody(t, handler, jellyfinDelivery(t, captions, 5), "/Stream.vtt", "WEBVTT", "Door closes")
	assertJellyfinDirectSubtitle(t, fixture.jellyfinSubtitleSource(t, handler, token, itemID, `[]`, -1, 1), 1)
	subrip := fixture.jellyfinSubtitleSource(t, handler, token, itemID, `[{"Format":"srt","Method":"External"}]`, 3, 1)
	assertJellyfinDirectSubtitle(t, subrip, 1)
	assertJellyfinSubtitleBody(t, handler, jellyfinDelivery(t, subrip, 3), "/Stream.srt", "1\n00:00:00,000 --> 00:00:01,000\nDoor closes", "")
	assertJellyfinSubtitleBody(t, handler, jellyfinDelivery(t, subrip, sidecar), "/Stream.srt", "1\n00:00:00,000 --> 00:00:01,000\nCafé crème", "")
	for name, profiles := range map[string]string{"no subtitle support": `[]`, "image subtitle": `[{"Format":"vtt","Method":"External"}]`} {
		stream := map[string]int{"no subtitle support": 3, "image subtitle": 4}[name]
		burned := fixture.jellyfinSubtitleSource(t, handler, token, itemID, profiles, stream, 1)
		if burned.SupportsDirectPlay || burned.TranscodingURL == "" {
			t.Fatalf("%s: source = %+v, want burn-in transcode", name, burned)
		}
	}
}

func (fixture EmbeddedSubtitleFixture) jellyfinSubtitleServer(t *testing.T) http.Handler {
	t.Helper()
	media, data, cache, tools := t.TempDir(), t.TempDir(), CacheDir(t), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Film.fr.srt"), []byte("1\r\n00:00:00,000 --> 00:00:01,000\r\nCaf\xe9 cr\xe8me\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac","disposition":{"default":1},"tags":{"language":"eng"}},{"index":2,"codec_type":"audio","codec_name":"aac","tags":{"language":"fra"}},{"index":3,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"}},{"index":4,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","tags":{"language":"eng"}},{"index":5,"codec_type":"subtitle","codec_name":"eia_608","tags":{"language":"eng"}}],"format":{"format_name":"matroska"}}'
`)
	ffmpeg := filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, "#!/bin/sh\nprintf 'WEBVTT\\n\\n00:00.000 --> 00:01.000\\nDoor closes\\n'\n")
	return fixture.New(t, AutomaticSkipConfig{MediaDir: media, DataDir: data, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg, Jellyfin: true})
}

func (fixture EmbeddedSubtitleFixture) jellyfinSubtitleSource(t *testing.T, handler http.Handler, token, itemID, subtitles string, subtitle, audio int) jellyfinSubtitleSource {
	t.Helper()
	body := fmt.Sprintf(`{"SubtitleStreamIndex":%d,"AudioStreamIndex":%d,"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video","Container":"mkv","VideoCodec":"h264","AudioCodec":"aac"}],"TranscodingProfiles":[{"Type":"Video","Container":"mp4","VideoCodec":"h264","AudioCodec":"aac","Protocol":"hls"}],"SubtitleProfiles":%s}}`, subtitle, audio, subtitles)
	response := fixture.Call(t, handler, http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", body, token)
	var result struct{ MediaSources []jellyfinSubtitleSource }
	fixture.Decode(t, response, &result)
	if response.Code != http.StatusOK || len(result.MediaSources) != 1 {
		t.Fatalf("PlaybackInfo %s = %d %q", body, response.Code, response.Body.String())
	}
	return result.MediaSources[0]
}

func assertJellyfinDirectSubtitle(t *testing.T, source jellyfinSubtitleSource, audio int) {
	t.Helper()
	if !source.SupportsDirectPlay || source.TranscodingURL != "" || source.DefaultAudioStreamIndex != audio {
		t.Fatalf("source = %+v, want direct play with audio stream %d", source, audio)
	}
}

func assertUniqueJellyfinStreamIndexes(t *testing.T, source jellyfinSubtitleSource) {
	t.Helper()
	seen := map[int]bool{}
	for _, stream := range source.MediaStreams {
		if seen[stream.Index] {
			t.Fatalf("duplicate stream index %d in %+v", stream.Index, source.MediaStreams)
		}
		seen[stream.Index] = true
	}
}

func jellyfinExternalIndex(t *testing.T, source jellyfinSubtitleSource) int {
	t.Helper()
	for _, stream := range source.MediaStreams {
		if stream.Type == "Subtitle" && stream.IsExternal {
			return stream.Index
		}
	}
	t.Fatalf("no external subtitle in %+v", source.MediaStreams)
	return 0
}

func jellyfinDelivery(t *testing.T, source jellyfinSubtitleSource, index int) string {
	t.Helper()
	for _, stream := range source.MediaStreams {
		if stream.Type == "Subtitle" && stream.Index == index && stream.DeliveryURL != "" {
			return stream.DeliveryURL
		}
	}
	t.Fatalf("no delivery URL for subtitle %d in %+v", index, source.MediaStreams)
	return ""
}

func assertJellyfinSubtitleBody(t *testing.T, handler http.Handler, delivery, suffix, prefix, contains string) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, delivery, nil))
	body := strings.ReplaceAll(response.Body.String(), "\r\n", "\n")
	path, _, _ := strings.Cut(delivery, "?")
	if response.Code != http.StatusOK || !strings.HasSuffix(path, suffix) || !strings.HasPrefix(body, prefix) || !strings.Contains(body, contains) || !utf8.ValidString(body) || strings.HasPrefix(body, "\xef\xbb\xbf") {
		t.Fatalf("subtitle %s = %d %q", delivery, response.Code, response.Body.String())
	}
	if suffix == "/Stream.srt" && (strings.Contains(body, "WEBVTT") || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/x-subrip")) {
		t.Fatalf("SubRip delivery = %q %q", response.Header().Get("Content-Type"), body)
	}
}
