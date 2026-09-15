package server_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestJellyfinPlaybackPathCarriesCapabilityForHeaderlessMediaRequests(t *testing.T) {
	t.Parallel()
	handler, token, _ := jellyfinTestServer(t)
	movies := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId=00000000000000000000000000000001", "", token)
	var result jellyfinItems
	decodeJellyfin(t, movies, &result)
	if len(result.Items) != 1 {
		t.Fatalf("movies = %d %q", movies.Code, movies.Body.String())
	}

	playback := jellyfinCall(t, handler, http.MethodPost, "/Items/"+result.Items[0].ID+"/PlaybackInfo", `{}`, token)
	var delivery struct {
		PlaySessionID string `json:"PlaySessionId"`
		MediaSources  []struct {
			Path string `json:"Path"`
		} `json:"MediaSources"`
	}
	decodeJellyfin(t, playback, &delivery)
	if playback.Code != http.StatusOK || delivery.PlaySessionID == "" || len(delivery.MediaSources) != 1 {
		t.Fatalf("playback = %d %q", playback.Code, playback.Body.String())
	}
	streamURL, err := url.Parse(delivery.MediaSources[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if streamURL.Path != "/Videos/"+result.Items[0].ID+"/stream" || streamURL.Query().Get("playSessionId") != delivery.PlaySessionID || streamURL.Query().Get("api_key") != token {
		t.Fatalf("media source path = %q", delivery.MediaSources[0].Path)
	}
	assertHeaderlessJellyfinStreams(t, handler, delivery.MediaSources[0].Path, result.Items[0].ID, delivery.PlaySessionID)
}

func TestJellyfinMediaURLAcceptsStandardQueryToken(t *testing.T) {
	t.Parallel()
	handler, token, _ := jellyfinTestServer(t)
	movies := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId=00000000000000000000000000000001", "", token)
	var result jellyfinItems
	decodeJellyfin(t, movies, &result)
	if len(result.Items) != 1 {
		t.Fatalf("movies = %d %q", movies.Code, movies.Body.String())
	}
	stream := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+result.Items[0].ID+"/stream?api_key="+token, "", "")
	if stream.Code != http.StatusOK {
		t.Fatalf("query-token media request = %d %q", stream.Code, stream.Body.String())
	}
	for _, query := range []string{"", "api_key=", "api_key=" + strings.Repeat("a", 257), "api_key=" + token + "&api_key=" + token} {
		request := "/Videos/" + result.Items[0].ID + "/stream"
		if query != "" {
			request += "?" + query
		}
		response := jellyfinCall(t, handler, http.MethodGet, request, "", "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid query token %q = %d %q", query, response.Code, response.Body.String())
		}
	}
	playback := jellyfinCall(t, handler, http.MethodPost, "/Items/"+result.Items[0].ID+"/PlaybackInfo", `{}`, token)
	var delivery struct {
		PlaySessionID string `json:"PlaySessionId"`
	}
	decodeJellyfin(t, playback, &delivery)
	ambiguous := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+result.Items[0].ID+"/stream?playSessionId="+delivery.PlaySessionID+"&api_key="+token+"&api_key="+token, "", "")
	if ambiguous.Code != http.StatusUnauthorized {
		t.Fatalf("ambiguous query with play session = %d %q", ambiguous.Code, ambiguous.Body.String())
	}
}

func TestJellyfinPlaybackURLSurvivesServerRestart(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true}
	handler := newJellyfinServer(t, config)
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := jellyfinLogin(t, handler, owner)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items", "", token)
	var library jellyfinItems
	decodeJellyfin(t, items, &library)
	if len(library.Items) != 1 {
		t.Fatalf("items = %d %q", items.Code, items.Body.String())
	}
	playback := jellyfinCall(t, handler, http.MethodPost, "/Items/"+library.Items[0].ID+"/PlaybackInfo", `{}`, token)
	var delivery struct {
		PlaySessionID string `json:"PlaySessionId"`
		MediaSources  []struct {
			Path string `json:"Path"`
		} `json:"MediaSources"`
	}
	decodeJellyfin(t, playback, &delivery)
	if playback.Code != http.StatusOK || len(delivery.MediaSources) != 1 {
		t.Fatalf("playback = %d %q", playback.Code, playback.Body.String())
	}
	streamURL, err := url.Parse(delivery.MediaSources[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if streamURL.Query().Get("playSessionId") != delivery.PlaySessionID || streamURL.Query().Get("api_key") != token {
		t.Fatalf("stream query = %q", streamURL.RawQuery)
	}
	assertJellyfinCookieCapability(t, handler, library.Items[0].ID, owner)
	restarted := newJellyfinServer(t, config)
	stream := jellyfinCall(t, restarted, http.MethodGet, delivery.MediaSources[0].Path, "", "")
	if stream.Code != http.StatusOK {
		t.Fatalf("stream after restart = %d %q", stream.Code, stream.Body.String())
	}
}

func assertHeaderlessJellyfinStreams(t *testing.T, handler http.Handler, sourcePath, id, playID string) {
	t.Helper()
	stream := jellyfinCall(t, handler, http.MethodGet, sourcePath, "", "")
	if stream.Code != http.StatusOK {
		t.Fatalf("headerless media request = %d %q", stream.Code, stream.Body.String())
	}
	sourceStream := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+id+"/"+id+"/stream?playSessionId="+playID, "", "")
	if sourceStream.Code != http.StatusOK {
		t.Fatalf("source-qualified media request = %d %q", sourceStream.Code, sourceStream.Body.String())
	}
	withoutSession := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+id+"/"+id+"/stream", "", "")
	if withoutSession.Code != http.StatusUnauthorized {
		t.Fatalf("source-qualified media without session = %d %q", withoutSession.Code, withoutSession.Body.String())
	}
}

func assertJellyfinCookieCapability(t *testing.T, handler http.Handler, id string, owner *http.Cookie) {
	t.Helper()
	cookiePlayback := requestWithCookie(t, handler, http.MethodGet, "/Items/"+id+"/PlaybackInfo", "", owner)
	var cookieDelivery struct {
		MediaSources []struct {
			Path string `json:"Path"`
		} `json:"MediaSources"`
	}
	decodeJellyfin(t, cookiePlayback, &cookieDelivery)
	if cookiePlayback.Code != http.StatusOK || len(cookieDelivery.MediaSources) != 1 {
		t.Fatalf("cookie playback = %d %q", cookiePlayback.Code, cookiePlayback.Body.String())
	}
	cookieURL, err := url.Parse(cookieDelivery.MediaSources[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if cookieURL.Query().Get("api_key") != "" || strings.Contains(cookiePlayback.Body.String(), owner.Value) {
		t.Fatalf("cookie token projected into media URL: %q", cookieDelivery.MediaSources[0].Path)
	}
	ambiguousCookie := requestWithCookie(t, handler, http.MethodGet, cookieDelivery.MediaSources[0].Path+"&api_key=x&api_key=x", "", owner)
	if ambiguousCookie.Code != http.StatusUnauthorized {
		t.Fatalf("ambiguous query with cookie = %d %q", ambiguousCookie.Code, ambiguousCookie.Body.String())
	}
}
