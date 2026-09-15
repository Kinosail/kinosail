package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestJellyseerrJellyfinSeamSupportsDiscoveryLookupAndOwnerAPIKey(t *testing.T) { //nolint:cyclop,gocognit,funlen // One client contract covers the setup sequence and all dependent reads.
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	writeJellyseerrFixture(t, mediaDir)
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "Jellyseerr test", "totp": true})
	var enrollment struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	mustJSON(t, setup, &enrollment)
	if setup.Code != http.StatusCreated || enrollment.Token == "" || enrollment.TOTP.Secret == "" {
		t.Fatalf("API setup = %d %q", setup.Code, setup.Body.String())
	}
	confirmed := apiCall(t, handler, enrollment.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, enrollment.TOTP.Secret, time.Now())})
	if confirmed.Code != http.StatusOK {
		t.Fatalf("MFA confirmation = %d %q", confirmed.Code, confirmed.Body.String())
	}
	native := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password", "device": "Jellyseerr native auth", "code": testTOTP(t, enrollment.TOTP.Secret, time.Now())})
	var session struct {
		Token string `json:"token"`
	}
	mustJSON(t, native, &session)
	if native.Code != http.StatusCreated || session.Token == "" {
		t.Fatalf("native Kinosail auth = %d %q", native.Code, native.Body.String())
	}
	ownerToken := session.Token
	disableTestMFA(t, handler, ownerToken)
	created := apiCall(t, handler, ownerToken, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password", "libraries": []string{"all"}})
	if created.Code != http.StatusCreated {
		t.Fatalf("create Viewer Profile = %d %q", created.Code, created.Body.String())
	}

	info := jellyfinCall(t, handler, http.MethodGet, "/System/Info", "", ownerToken)
	if info.Code != http.StatusOK || !strings.Contains(info.Body.String(), `"ServerName":"Kinosail"`) {
		t.Fatalf("system info = %d %q", info.Code, info.Body.String())
	}
	users := jellyfinCall(t, handler, http.MethodGet, "/Users", "", ownerToken)
	if users.Code != http.StatusOK || !strings.Contains(users.Body.String(), `"Name":"Viewer"`) {
		t.Fatalf("users = %d %q", users.Code, users.Body.String())
	}
	folders := jellyfinCall(t, handler, http.MethodGet, "/Library/MediaFolders", "", ownerToken)
	var folderResult struct {
		Items []struct {
			ID string `json:"Id"`
		} `json:"Items"`
	}
	decodeJellyfin(t, folders, &folderResult)
	if folders.Code != http.StatusOK || len(folderResult.Items) < 2 {
		t.Fatalf("media folders = %d %q", folders.Code, folders.Body.String())
	}

	items := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId="+folderResult.Items[0].ID+"&IncludeItemTypes=Movie", "", ownerToken)
	var itemResult jellyfinItems
	decodeJellyfin(t, items, &itemResult)
	if items.Code != http.StatusOK || len(itemResult.Items) != 1 {
		t.Fatalf("movie items = %d %q", items.Code, items.Body.String())
	}
	movieID := itemResult.Items[0].ID
	byID := jellyfinCall(t, handler, http.MethodGet, "/Items?ids="+movieID+"&fields=ProviderIds,MediaSources,Width,Height,IsHD,DateCreated", "", ownerToken)
	decodeJellyfin(t, byID, &itemResult)
	if byID.Code != http.StatusOK || len(itemResult.Items) != 1 || itemResult.Items[0].ID != movieID || !strings.Contains(byID.Body.String(), `"Tmdb":"329865"`) {
		t.Fatalf("item lookup = %d %q", byID.Code, byID.Body.String())
	}

	shows := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId="+folderResult.Items[1].ID+"&IncludeItemTypes=Series", "", ownerToken)
	decodeJellyfin(t, shows, &itemResult)
	if shows.Code != http.StatusOK || len(itemResult.Items) != 1 || !strings.Contains(shows.Body.String(), `"Type":"Series"`) {
		t.Fatalf("show items = %d %q", shows.Code, shows.Body.String())
	}
	showID := itemResult.Items[0].ID
	seasons := jellyfinCall(t, handler, http.MethodGet, "/Shows/"+showID+"/Seasons", "", ownerToken)
	var seasonResult struct {
		Items []struct {
			ID string `json:"Id"`
		} `json:"Items"`
	}
	decodeJellyfin(t, seasons, &seasonResult)
	if seasons.Code != http.StatusOK || len(seasonResult.Items) != 1 || !strings.Contains(seasons.Body.String(), `"Type":"Season"`) {
		t.Fatalf("seasons = %d %q", seasons.Code, seasons.Body.String())
	}
	episodes := jellyfinCall(t, handler, http.MethodGet, "/Shows/"+showID+"/Episodes?seasonId="+seasonResult.Items[0].ID, "", ownerToken)
	if episodes.Code != http.StatusOK || !strings.Contains(episodes.Body.String(), `"Type":"Episode"`) {
		t.Fatalf("episodes = %d %q", episodes.Code, episodes.Body.String())
	}

	key := jellyfinCall(t, handler, http.MethodPost, "/Auth/Keys?App=Seerr", "", ownerToken)
	if key.Code != http.StatusNoContent {
		t.Fatalf("create Seerr API key = %d %q", key.Code, key.Body.String())
	}
	keys := jellyfinCall(t, handler, http.MethodGet, "/Auth/Keys", "", ownerToken)
	var keyResult struct {
		Items []struct {
			App   string `json:"AppName"`
			Token string `json:"AccessToken"`
		} `json:"Items"`
	}
	decodeJellyfin(t, keys, &keyResult)
	if keys.Code != http.StatusOK || len(keyResult.Items) != 1 || keyResult.Items[0].App != "Seerr" || keyResult.Items[0].Token == "" {
		t.Fatalf("API keys = %d %q", keys.Code, keys.Body.String())
	}
	apiToken := keyResult.Items[0].Token
	if library := jellyfinCall(t, handler, http.MethodGet, "/Library/MediaFolders", "", apiToken); library.Code != http.StatusOK {
		t.Fatalf("Seerr API key folders = %d %q", library.Code, library.Body.String())
	}
	if listed := apiCall(t, handler, ownerToken, http.MethodGet, "/api/v1/api-keys", nil); listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"expires":"Never"`) {
		t.Fatalf("persistent API key listing = %d %q", listed.Code, listed.Body.String())
	}

	invalidApp := jellyfinCall(t, handler, http.MethodPost, "/Auth/Keys?App=unknown", "", ownerToken)
	duplicateApp := jellyfinCall(t, handler, http.MethodPost, "/Auth/Keys?App=Seerr&App=Jellyseerr", "", ownerToken)
	invalidIDs := jellyfinCall(t, handler, http.MethodGet, "/Items?ids=movie,,other", "", ownerToken)
	oversizedIDs := jellyfinCall(t, handler, http.MethodGet, "/Items?ids="+strings.Repeat("x", 65), "", ownerToken)
	unchangedKeys := jellyfinCall(t, handler, http.MethodGet, "/Auth/Keys", "", ownerToken)
	var unchangedKeyResult struct {
		Items []struct {
			Token string `json:"AccessToken"`
		} `json:"Items"`
	}
	decodeJellyfin(t, unchangedKeys, &unchangedKeyResult)
	if invalidApp.Code != http.StatusBadRequest || duplicateApp.Code != http.StatusBadRequest || invalidIDs.Code != http.StatusBadRequest || oversizedIDs.Code != http.StatusBadRequest || len(unchangedKeyResult.Items) != 1 {
		t.Fatalf("invalid inputs app=%d duplicate=%d ids=%d oversized=%d keys=%d", invalidApp.Code, duplicateApp.Code, invalidIDs.Code, oversizedIDs.Code, len(unchangedKeyResult.Items))
	}

	viewer := jellyfinCall(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{"Username":"Viewer","Pw":"viewer-password"}`, "")
	var viewerLogin struct {
		Token string `json:"AccessToken"`
	}
	decodeJellyfin(t, viewer, &viewerLogin)
	if viewer.Code != http.StatusOK || viewerLogin.Token == "" {
		t.Fatalf("Viewer login = %d %q", viewer.Code, viewer.Body.String())
	}
	if viewerKeys := jellyfinCall(t, handler, http.MethodGet, "/Auth/Keys", "", viewerLogin.Token); viewerKeys.Code != http.StatusForbidden {
		t.Fatalf("Viewer API keys = %d %q", viewerKeys.Code, viewerKeys.Body.String())
	}

	restarted := server.New(trustedJellyfinConfig(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true}))
	if library := jellyfinCall(t, restarted, http.MethodGet, "/Library/MediaFolders", "", apiToken); library.Code != http.StatusOK {
		t.Fatalf("persistent Seerr API key = %d %q", library.Code, library.Body.String())
	}
}

func writeJellyseerrFixture(t *testing.T, mediaDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.nfo"), []byte(`<movie><title>Arrival</title><year>2016</year><uniqueid type="tmdb">329865</uniqueid><uniqueid type="imdb">tt2543164</uniqueid></movie>`), 0o600); err != nil {
		t.Fatal(err)
	}
	season := filepath.Join(mediaDir, "Severance (2014) {imdb-tt4574334}", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance.S01E01.Good.News.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
}
