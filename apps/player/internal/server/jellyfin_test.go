package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestJellyfinClientCanDiscoverAndAuthenticate(t *testing.T) { //nolint:cyclop // One sequential client contract.
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	public := jellyfinCall(t, handler, http.MethodGet, "/System/Info/Public", "", "")
	var info struct {
		ID         string `json:"Id"`
		ServerName string `json:"ServerName"`
		Version    string `json:"Version"`
		Product    string `json:"ProductName"`
	}
	decodeJellyfin(t, public, &info)
	if public.Code != http.StatusOK || len(info.ID) != 32 || info.ServerName != "Kinosail" || info.Version != "12.0.0" || info.Product != "Jellyfin Server" {
		t.Fatalf("public info = %d %q", public.Code, public.Body.String())
	}
	for _, path := range []string{"/QuickConnect/Enabled", "/Users/Public", "/Branding/Configuration"} {
		response := jellyfinCall(t, handler, http.MethodGet, path, "", "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
	unauthorized := jellyfinCall(t, handler, http.MethodGet, "/Items", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous Items = %d %q", unauthorized.Code, unauthorized.Body.String())
	}
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, userID := jellyfinLogin(t, handler, owner)
	me := jellyfinCall(t, handler, http.MethodGet, "/Users/Me", "", token)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"Id":"`+userID+`"`) || !strings.Contains(me.Body.String(), `"AuthenticationProviderId":"Kinosail"`) {
		t.Fatalf("me = %d %q", me.Code, me.Body.String())
	}
	alternateHeaderRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Items", nil)
	alternateHeaderRequest.Header.Set("X-Emby-Authorization", `MediaBrowser Token="`+token+`", Client="TestClient", Version="8.5.1", Device="iPhone", DeviceId="test"`)
	alternateHeader := httptest.NewRecorder()
	handler.ServeHTTP(alternateHeader, alternateHeaderRequest)
	if alternateHeader.Code != http.StatusOK {
		t.Fatalf("Alternate header authorization = %d %q", alternateHeader.Code, alternateHeader.Body.String())
	}
	restarted := server.New(trustedJellyfinConfig(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true}))
	public = jellyfinCall(t, restarted, http.MethodGet, "/System/Info/Public", "", "")
	var restartedInfo struct {
		ID string `json:"Id"`
	}
	decodeJellyfin(t, public, &restartedInfo)
	if restartedInfo.ID != info.ID {
		t.Fatalf("server ID changed: %q != %q", restartedInfo.ID, info.ID)
	}
}

func TestJellyfinPasswordLoginDoesNotGrantOwnerAdministration(t *testing.T) {
	t.Parallel()
	handler, token := apiServer(t)
	enableJellyfin(t, handler, token)
	disableTestMFA(t, handler, token)
	assertAPICalls(t, handler, token, []apiTestCall{
		{Method: http.MethodPost, Path: "/api/v1/profiles", Body: map[string]any{"name": "Partner", "password": "partner-password", "owner": true}, Status: http.StatusCreated},
		{Method: http.MethodPost, Path: "/api/v1/profiles", Body: map[string]any{"name": "Guest", "password": "guest-password"}, Status: http.StatusCreated},
	})
	owner := jellyfinCall(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{"Username":"Partner","Pw":"partner-password"}`, "")
	viewer := jellyfinCall(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{"Username":"Guest","Pw":"guest-password"}`, "")
	if owner.Code != http.StatusOK || !strings.Contains(owner.Body.String(), `"IsAdministrator":false`) || !strings.Contains(owner.Body.String(), `"EnableContentDownloading":true`) {
		t.Fatalf("Jellyfin Owner = %d %q", owner.Code, owner.Body.String())
	}
	if viewer.Code != http.StatusOK || !strings.Contains(viewer.Body.String(), `"IsAdministrator":false`) {
		t.Fatalf("Jellyfin Viewer = %d %q", viewer.Code, viewer.Body.String())
	}
	if !strings.Contains(viewer.Body.String(), `"EnableRemoteAccess":false`) || !strings.Contains(viewer.Body.String(), `"EnableContentDownloading":false`) {
		t.Fatalf("Jellyfin Viewer policy = %d %q", viewer.Code, viewer.Body.String())
	}
}

func TestJellyfinPlaybackCapabilityUsesCurrentProfilePolicy(t *testing.T) {
	t.Parallel()
	handler, ownerToken := apiServer(t)
	enableJellyfin(t, handler, ownerToken)
	disableTestMFA(t, handler, ownerToken)
	created := apiCall(t, handler, ownerToken, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password", "rating": "all", "libraries": []string{"all"}})
	var profile struct {
		ID string `json:"id"`
	}
	mustJSON(t, created, &profile)
	login := jellyfinCall(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{"Username":"Viewer","Pw":"viewer-password"}`, "")
	var session struct {
		Token string `json:"AccessToken"`
	}
	decodeJellyfin(t, login, &session)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items", "", session.Token)
	var library jellyfinItems
	decodeJellyfin(t, items, &library)
	if len(library.Items) == 0 {
		t.Fatalf("Viewer library = %d %q", items.Code, items.Body.String())
	}
	id := library.Items[0].ID
	playback := jellyfinCall(t, handler, http.MethodPost, "/Items/"+id+"/PlaybackInfo", `{}`, session.Token)
	var play struct {
		ID string `json:"PlaySessionId"`
	}
	decodeJellyfin(t, playback, &play)
	before := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+id+"/stream?playSessionId="+play.ID, "", "")
	removed := apiCall(t, handler, ownerToken, http.MethodDelete, "/api/v1/profiles/"+profile.ID, nil)
	after := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+id+"/stream?playSessionId="+play.ID, "", "")
	if before.Code != http.StatusOK || removed.Code != http.StatusNoContent || after.Code != http.StatusNotFound {
		t.Fatalf("playback capability before=%d remove=%d after=%d", before.Code, removed.Code, after.Code)
	}
}

func TestJellyfinClientCanBrowseMoviesAndShows(t *testing.T) {
	t.Parallel()

	handler, token, mediaDir := jellyfinTestServer(t)
	views := jellyfinCall(t, handler, http.MethodGet, "/UserViews", "", token)
	if views.Code != http.StatusOK || !strings.Contains(views.Body.String(), `"CollectionType":"movies"`) || !strings.Contains(views.Body.String(), `"CollectionType":"tvshows"`) {
		t.Fatalf("views = %d %q", views.Code, views.Body.String())
	}
	checkJellyfinMovies(t, handler, token, mediaDir)
	checkJellyfinShow(t, handler, token)
}

func TestJellyfinClientDoesNotReceivePlaybackSegmentsAlreadyOmittedFromTheStream(t *testing.T) {
	t.Parallel()
	mediaDir, toolsDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Episode.S01E01.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(toolsDir, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264"}],"chapters":[{"start_time":"12","end_time":"72","tags":{"title":"Opening Credits"}},{"start_time":"1200","end_time":"1260","tags":{"title":"End Credits"}}],"format":{"duration":"1260"}}'
`)
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, CacheDir: t.TempDir(), FFprobe: ffprobe, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := jellyfinLogin(t, handler, owner)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items?IncludeItemTypes=Episode", "", token)
	var result jellyfinItems
	decodeJellyfin(t, items, &result)
	if len(result.Items) != 1 {
		t.Fatalf("items = %q", items.Body.String())
	}
	segments := jellyfinRequest(t, handler, token, http.MethodGet, "/MediaSegments/"+result.Items[0].ID, "Jellyfin for Android", "2.7.0", "")
	if segments.Code != http.StatusOK || !strings.Contains(segments.Body.String(), `"Items":[]`) {
		t.Fatalf("already omitted segments = %d %q", segments.Code, segments.Body.String())
	}
}

func checkJellyfinMovies(t *testing.T, handler http.Handler, token, mediaDir string) {
	t.Helper()
	movies := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId=00000000000000000000000000000001&IncludeItemTypes=Movie", "", token)
	var movieResult jellyfinItems
	decodeJellyfin(t, movies, &movieResult)
	if len(movieResult.Items) != 1 || movieResult.Items[0].Name != "Arrival" || len(movieResult.Items[0].ID) != 32 || strings.Contains(movies.Body.String(), mediaDir) {
		t.Fatalf("movies = %d %q", movies.Code, movies.Body.String())
	}
}

func checkJellyfinShow(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	series := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId=00000000000000000000000000000002&IncludeItemTypes=Series", "", token)
	var seriesResult jellyfinItems
	decodeJellyfin(t, series, &seriesResult)
	if len(seriesResult.Items) != 1 || seriesResult.Items[0].Name != "Severance" || len(seriesResult.Items[0].ID) != 32 {
		t.Fatalf("series = %d %q", series.Code, series.Body.String())
	}
	seasons := jellyfinCall(t, handler, http.MethodGet, "/Shows/"+seriesResult.Items[0].ID+"/Seasons", "", token)
	var seasonResult jellyfinItems
	decodeJellyfin(t, seasons, &seasonResult)
	if len(seasonResult.Items) != 1 || seasonResult.Items[0].Name != "Season 1" || len(seasonResult.Items[0].ID) != 32 {
		t.Fatalf("seasons = %d %q", seasons.Code, seasons.Body.String())
	}
	episodes := jellyfinCall(t, handler, http.MethodGet, "/Shows/"+seriesResult.Items[0].ID+"/Episodes?SeasonId="+seasonResult.Items[0].ID, "", token)
	if episodes.Code != http.StatusOK || !strings.Contains(episodes.Body.String(), `"Type":"Episode"`) || !strings.Contains(episodes.Body.String(), `"IndexNumber":1`) {
		t.Fatalf("episodes = %d %q", episodes.Code, episodes.Body.String())
	}
}

func TestJellyfinClientCanPlayAndSyncProgress(t *testing.T) { //nolint:cyclop // One sequential client contract.
	t.Parallel()

	handler, token, _ := jellyfinTestServer(t)
	movies := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId=00000000000000000000000000000001", "", token)
	var result jellyfinItems
	decodeJellyfin(t, movies, &result)
	id := result.Items[0].ID
	anonymous := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+id+"/stream", "", "")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous stream = %d %q", anonymous.Code, anonymous.Body.String())
	}
	playback := jellyfinCall(t, handler, http.MethodPost, "/Items/"+id+"/PlaybackInfo", `{}`, token)
	var playbackInfo struct {
		PlaySessionID string `json:"PlaySessionId"`
	}
	decodeJellyfin(t, playback, &playbackInfo)
	if playback.Code != http.StatusOK || playbackInfo.PlaySessionID == "" || !strings.Contains(playback.Body.String(), `"Id":"`+id+`"`) {
		t.Fatalf("playback = %d %q", playback.Code, playback.Body.String())
	}
	stream := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Videos/"+id+"/stream?playSessionId="+playbackInfo.PlaySessionID, nil)
	stream.Header.Set("Range", "bytes=2-5")
	media := httptest.NewRecorder()
	handler.ServeHTTP(media, stream)
	if media.Code != http.StatusPartialContent || media.Body.String() != "2345" {
		t.Fatalf("stream = %d %q", media.Code, media.Body.String())
	}
	progress := jellyfinCall(t, handler, http.MethodPost, "/Sessions/Playing/Progress", fmt.Sprintf(`{"ItemId":%q,"PositionTicks":420000000}`, id), token)
	item := jellyfinCall(t, handler, http.MethodGet, "/Items/"+id, "", token)
	if progress.Code != http.StatusNoContent || !strings.Contains(item.Body.String(), `"PlaybackPositionTicks":420000000`) {
		t.Fatalf("progress = %d %q, item = %d %q", progress.Code, progress.Body.String(), item.Code, item.Body.String())
	}
	image := jellyfinCall(t, handler, http.MethodGet, "/Items/"+id+"/Images/Primary", "", token)
	subtitle := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+id+"/"+id+"/Subtitles/0/Stream.vtt?playSessionId="+playbackInfo.PlaySessionID, "", "")
	if image.Body.String() != "poster" || !strings.Contains(subtitle.Body.String(), "WEBVTT") {
		t.Fatalf("image = %d %q, subtitle = %d %q", image.Code, image.Body.String(), subtitle.Code, subtitle.Body.String())
	}
	favorite := jellyfinCall(t, handler, http.MethodPost, "/UserFavoriteItems/"+id, "", token)
	played := jellyfinCall(t, handler, http.MethodPost, "/UserPlayedItems/"+id, "", token)
	if !strings.Contains(favorite.Body.String(), `"IsFavorite":true`) || !strings.Contains(played.Body.String(), `"Played":true`) {
		t.Fatalf("favorite = %d %q, played = %d %q", favorite.Code, favorite.Body.String(), played.Code, played.Body.String())
	}
}

type jellyfinItems struct {
	Items []struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	} `json:"Items"`
}

func jellyfinTestServer(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(mediaDir, "Arrival.mp4"):                  "0123456789",
		filepath.Join(mediaDir, "Arrival.jpg"):                  "poster",
		filepath.Join(mediaDir, "Arrival.srt"):                  "1\n00:00:00,000 --> 00:00:01,000\nHello\n",
		filepath.Join(season, "Severance.S01E01.Good.News.mkv"): "episode",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := jellyfinLogin(t, handler, owner)
	return handler, token, mediaDir
}

func jellyfinLogin(t *testing.T, handler http.Handler, owner *http.Cookie) (string, string) {
	t.Helper()
	initiateRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Initiate", nil)
	initiateRequest.Header.Set("Authorization", `MediaBrowser Client="Jellyfin Mobile", Device="Test", DeviceId="test", Version="2"`)
	initiate := httptest.NewRecorder()
	handler.ServeHTTP(initiate, initiateRequest)
	var state struct{ Secret, Code string }
	decodeJellyfin(t, initiate, &state)
	approved := requestWithCookie(t, handler, http.MethodPost, "/QuickConnect/Authorize?Code="+state.Code, "", owner)
	if initiate.Code != http.StatusOK || approved.Code != http.StatusOK {
		t.Fatalf("Quick Connect = %d %q, approval = %d %q", initiate.Code, initiate.Body.String(), approved.Code, approved.Body.String())
	}
	response := jellyfinCall(t, handler, http.MethodPost, "/Users/AuthenticateWithQuickConnect", `{"Secret":"`+state.Secret+`"}`, "")
	var login struct {
		Token string `json:"AccessToken"`
		User  struct {
			ID string `json:"Id"`
		} `json:"User"`
	}
	decodeJellyfin(t, response, &login)
	if response.Code != http.StatusOK || login.Token == "" || login.User.ID == "" {
		t.Fatalf("login = %d %q", response.Code, response.Body.String())
	}
	return login.Token, login.User.ID
}
