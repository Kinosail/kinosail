package servertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EmbeddedSubtitleFixture retains each app's real media and authentication adapters.
type EmbeddedSubtitleFixture struct {
	New             func(*testing.T, AutomaticSkipConfig) http.Handler
	WebCaptionLabel string
	SignIn          func(*testing.T, http.Handler, string, string) *http.Cookie
	Login           func(*testing.T, http.Handler, *http.Cookie) (string, string)
	WebCall         AuthCookieRequest
	Call            func(*testing.T, http.Handler, string, string, string, string) *httptest.ResponseRecorder
	Decode          func(*testing.T, *httptest.ResponseRecorder, any)
}

// TextCaptionsAcrossAdapters proves semantic captions and real extraction on every adapter.
func (fixture EmbeddedSubtitleFixture) TextCaptionsAcrossAdapters(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler, arguments := fixture.embeddedCaptionServer(t)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := fixture.Login(t, handler, owner)
	items := fixture.Call(t, handler, http.MethodGet, "/Items", "", token)
	var catalog struct{ Items []struct{ ID string } }
	fixture.Decode(t, items, &catalog)
	itemID, rawID := catalog.Items[0].ID, catalog.Items[0].ID[:16]
	assertEmbeddedCaptionAPI(t, handler, token, rawID)
	fixture.assertEmbeddedCaptionWeb(t, handler, owner, rawID, arguments)
	fixture.assertEmbeddedCaptionJellyfin(t, handler, token, itemID)
}

func (fixture EmbeddedSubtitleFixture) embeddedCaptionServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac","disposition":{"default":1}},{"index":3,"codec_type":"subtitle","codec_name":"subrip","disposition":{"default":1,"hearing_impaired":1},"tags":{"language":"eng","title":"English SDH"}}],"format":{"format_name":"matroska"}}'
`)
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\nprintf 'WEBVTT\\n\\n00:00.000 --> 00:01.000\\nDoor closes\\n'\n", arguments))
	return fixture.New(t, AutomaticSkipConfig{MediaDir: media, DataDir: data, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg, Jellyfin: true}), arguments
}

func assertEmbeddedCaptionAPI(t *testing.T, handler http.Handler, token, rawID string) {
	t.Helper()
	api := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+rawID+"/playback", nil)
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"role":"captions"`) || !strings.Contains(api.Body.String(), `"embedded":true`) || !strings.Contains(api.Body.String(), `/subtitle/`+rawID+`/embedded/3`) {
		t.Fatalf("API captions = %d %q", api.Code, api.Body.String())
	}
}

func (fixture EmbeddedSubtitleFixture) assertEmbeddedCaptionWeb(t *testing.T, handler http.Handler, owner *http.Cookie, rawID, arguments string) {
	t.Helper()
	page := fixture.WebCall(t, handler, http.MethodGet, "/watch/"+rawID, "", owner)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `kind="captions"`) || !strings.Contains(page.Body.String(), `label="`+fixture.WebCaptionLabel+`"`) {
		t.Fatalf("web captions = %d %q", page.Code, page.Body.String())
	}
	extracted := fixture.WebCall(t, handler, http.MethodGet, "/subtitle/"+rawID+"/embedded/3", "", owner)
	used, err := os.ReadFile(arguments)
	if extracted.Code != http.StatusOK || !strings.Contains(extracted.Body.String(), "Door closes") || err != nil || !strings.Contains(string(used), "-map 0:3") {
		t.Fatalf("extraction = %d %q arguments=%q err=%v", extracted.Code, extracted.Body.String(), used, err)
	}
}

func (fixture EmbeddedSubtitleFixture) assertEmbeddedCaptionJellyfin(t *testing.T, handler http.Handler, token, itemID string) {
	t.Helper()
	playback := fixture.Call(t, handler, http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", `{"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video","Container":"mkv","VideoCodec":"h264","AudioCodec":"aac"}],"SubtitleProfiles":[{"Format":"srt","Method":"External"}]}}`, token)
	var result struct {
		PlaySessionID string `json:"PlaySessionId"`
		MediaSources  []struct {
			MediaStreams []struct {
				Index       int
				Type        string
				DeliveryURL string `json:"DeliveryUrl"`
			}
		}
	}
	fixture.Decode(t, playback, &result)
	var delivery string
	for _, stream := range result.MediaSources[0].MediaStreams {
		if stream.Type == "Subtitle" && stream.Index == 3 {
			delivery = stream.DeliveryURL
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, delivery, nil))
	if delivery == "" || response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Door closes") {
		t.Fatalf("Jellyfin caption = %d %q delivery=%q playback=%q", response.Code, response.Body.String(), delivery, playback.Body.String())
	}
}
