package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestJellyfinClientCanUseExtendedBrowseAndPlaybackContracts(t *testing.T) { //nolint:cyclop,funlen,gocognit // One official-client workflow exercises related Jellyfin contracts end to end.
	media, data := t.TempDir(), t.TempDir()
	season1, season2 := filepath.Join(media, "Show", "Season 1"), filepath.Join(media, "Show", "Season 2")
	for _, directory := range []string{season1, season2} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(media, "Movie.mp4"):                     "movie",
		filepath.Join(media, "Movie.jpg"):                     "movie poster",
		filepath.Join(media, "Movie.en.srt"):                  "1\n00:00:00,000 --> 00:00:01,000\nHello\n",
		filepath.Join(media, "Song.mp3"):                      "audio",
		filepath.Join(media, "Vacation.jpg"):                  "photo",
		filepath.Join(media, "Show", "poster.jpg"):            "show poster",
		filepath.Join(season1, "Show.S01E01.Pilot.mkv"):       "episode one",
		filepath.Join(season1, "Show.S01E01.Pilot-thumb.jpg"): "episode art",
		filepath.Join(season2, "Show.S02E01.Return.mkv"):      "episode two",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := newJellyfinServer(t, server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	if response := jellyfinCall(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{`, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed authentication = %d %q", response.Code, response.Body.String())
	}
	token, _ := jellyfinLogin(t, handler, owner)

	root := jellyfinCall(t, handler, http.MethodGet, "/Items?IncludeItemTypes=Movie,Series,Audio,Photo&SearchTerm=o", "", token)
	var result struct {
		Items []struct {
			ID, Name, Type string
		}
	}
	decodeJellyfin(t, root, &result)
	ids := make(map[string]string)
	for _, item := range result.Items {
		ids[item.Type] = item.ID
	}
	for _, kind := range []string{"Movie", "Series", "Audio", "Photo"} {
		if ids[kind] == "" {
			t.Fatalf("root items lack %s: %s", kind, root.Body.String())
		}
	}

	seriesChildren := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId="+ids["Series"], "", token)
	var seasons jellyfinItems
	decodeJellyfin(t, seriesChildren, &seasons)
	if len(seasons.Items) != 2 {
		t.Fatalf("series children = %q", seriesChildren.Body.String())
	}
	seasonChildren := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId="+seasons.Items[0].ID, "", token)
	if !strings.Contains(seasonChildren.Body.String(), `"Type":"Episode"`) {
		t.Fatalf("season children = %q", seasonChildren.Body.String())
	}
	for _, path := range []string{
		"/Items/" + ids["Series"], "/Items/" + seasons.Items[0].ID,
		"/Shows/" + ids["Series"] + "/Episodes?Season=1", "/Items?ParentId=00000000000000000000000000000002&IncludeItemTypes=Episode",
		"/Items?IncludeItemTypes=Episode&StartIndex=999", "/Items/Latest?Limit=1", "/Shows/NextUp",
	} {
		if response := jellyfinCall(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/Shows/missing/Seasons", "/Shows/missing/Episodes", "/Items/missing", "/Items/missing/Images/Primary", "/Items/" + ids["Photo"] + "/PlaybackInfo"} {
		if response := jellyfinCall(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}

	playback := jellyfinCall(t, handler, http.MethodGet, "/Items/"+ids["Movie"]+"/PlaybackInfo", "", token)
	var play struct {
		PlaySessionID string `json:"PlaySessionId"`
	}
	decodeJellyfin(t, playback, &play)
	for _, path := range []string{
		"/Items/" + ids["Movie"] + "/File", "/Items/" + ids["Movie"] + "/Download", "/Audio/" + ids["Movie"] + "/stream",
		"/Items/" + ids["Series"] + "/Images/Primary", "/Items/" + seasons.Items[0].ID + "/Images/Primary",
	} {
		if response := jellyfinCall(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/Videos/" + ids["Movie"] + "/invalid?playSessionId=missing", "/Videos/" + ids["Movie"] + "/stream?playSessionId=missing",
		"/Videos/" + ids["Movie"] + "/" + ids["Movie"] + "/Subtitles/99/Stream.vtt?playSessionId=" + play.PlaySessionID,
	} {
		if response := jellyfinCall(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}

	if response := jellyfinCall(t, handler, http.MethodPost, "/Sessions/Playing", `{`, token); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed progress = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodPost, "/Sessions/Playing/Progress", `{"ItemId":"missing","PositionTicks":-1}`, token); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid progress = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodPost, "/Sessions/Playing/Progress", `{"ItemId":"`+ids["Movie"]+`","PositionTicks":100000000}`, token); response.Code != http.StatusNoContent {
		t.Fatalf("progress = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodGet, "/UserItems/Resume", "", token); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), ids["Movie"]) {
		t.Fatalf("resume = %d %q", response.Code, response.Body.String())
	}
	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, "/UserItems/" + ids["Movie"] + "/UserData", ""},
		{http.MethodPost, "/UserItems/" + ids["Movie"] + "/UserData", `{"PlaybackPositionTicks":200000000}`},
		{http.MethodDelete, "/UserPlayedItems/" + ids["Movie"], ""},
		{http.MethodDelete, "/UserFavoriteItems/" + ids["Movie"], ""},
		{http.MethodPost, "/Sessions/Capabilities", `{}`},
		{http.MethodPost, "/Sessions/Capabilities/Full", `{}`},
	} {
		if response := jellyfinCall(t, handler, call.method, call.path, call.body, token); response.Code >= 400 {
			t.Fatalf("%s %s = %d %q", call.method, call.path, response.Code, response.Body.String())
		}
	}
	if response := jellyfinCall(t, handler, http.MethodPost, "/UserItems/"+ids["Movie"]+"/UserData", `{`, token); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed user data = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodPost, "/Sessions/Playing/Stopped", `{"ItemId":"`+ids["Movie"]+`","PlaySessionId":"`+play.PlaySessionID+`"}`, token); response.Code != http.StatusNoContent {
		t.Fatalf("stopped = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodPost, "/Sessions/Logout", "", token); response.Code != http.StatusNoContent {
		t.Fatalf("logout = %d %q", response.Code, response.Body.String())
	}
}
