package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JellyfinOwnerSessions binds media-only Owner contracts to real app HTTP handlers.
type JellyfinOwnerSessions struct {
	NewHandler func(*testing.T, string, string) http.Handler
	Set        func(string, string, string) error
	SignIn     func(*testing.T, http.Handler, string, string) *http.Cookie
	WebCall    func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
	Call       func(*testing.T, http.Handler, string, string, string, string) *httptest.ResponseRecorder
	Decode     func(*testing.T, *httptest.ResponseRecorder, any)
	APIServer  func(*testing.T) (http.Handler, string)
	Enable     func(*testing.T, http.Handler, string)
	DisableMFA func(*testing.T, http.Handler, string)
}

type jellyfinOwnerSession struct {
	Token string `json:"AccessToken"`
	User  struct {
		Policy struct {
			Administrator bool `json:"IsAdministrator"`
			Download      bool `json:"EnableContentDownloading"`
		} `json:"Policy"`
	} `json:"User"`
}

// JellyfinQuickConnectProjectsOwnerAsMediaViewer checks the approved media-only session.
func (fixture JellyfinOwnerSessions) JellyfinQuickConnectProjectsOwnerAsMediaViewer(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler(t, "", "")
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	initiateRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Initiate", nil)
	initiateRequest.Header.Set("Authorization", `MediaBrowser Client="Swiftfin iOS", Device="iPhone", DeviceId="phone-1", Version="1.3"`)
	initiate := httptest.NewRecorder()
	handler.ServeHTTP(initiate, initiateRequest)
	var state struct{ Secret, Code string }
	fixture.Decode(t, initiate, &state)
	approved := fixture.WebCall(t, handler, http.MethodPost, "/QuickConnect/Authorize?Code="+state.Code, "", owner)
	login := fixture.Call(t, handler, http.MethodPost, "/Users/AuthenticateWithQuickConnect", `{"Secret":"`+state.Secret+`"}`, "")
	var session jellyfinOwnerSession
	fixture.Decode(t, login, &session)
	if initiate.Code != http.StatusOK || approved.Code != http.StatusOK || login.Code != http.StatusOK || session.Token == "" || session.User.Policy.Administrator || !session.User.Policy.Download {
		t.Fatalf("restricted Owner Quick Connect: initiate=%d approve=%d login=%d %q", initiate.Code, approved.Code, login.Code, login.Body.String())
	}
	fixture.assertRestrictedOwnerRoutes(t, handler, session.Token, "restricted Owner Quick Connect")
}

// JellyfinPasswordSessionProjectsOwnerAsMediaViewer checks playback and persisted restrictions.
func (fixture JellyfinOwnerSessions) JellyfinPasswordSessionProjectsOwnerAsMediaViewer(t *testing.T) {
	t.Helper()
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := fixture.Set(dataDir, "security.require_mfa", "false"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(t, mediaDir, dataDir)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password"})
	if setup.Code != http.StatusCreated {
		t.Fatalf("setup = %d %q", setup.Code, setup.Body.String())
	}
	login := fixture.Call(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{"Username":"Owner","Pw":"owner-password"}`, "")
	var session jellyfinOwnerSession
	fixture.Decode(t, login, &session)
	if login.Code != http.StatusOK || session.Token == "" || session.User.Policy.Administrator || !session.User.Policy.Download {
		t.Fatalf("restricted Owner login = %d %q", login.Code, login.Body.String())
	}

	fixture.assertOwnerMediaPlayback(t, handler, session.Token)
	fixture.assertRestrictedOwnerRoutes(t, handler, session.Token, "restricted Owner")

	restarted := fixture.NewHandler(t, mediaDir, dataDir)
	if response := fixture.Call(t, restarted, http.MethodGet, "/Users/Me", "", session.Token); response.Code != http.StatusOK {
		t.Fatalf("persisted restricted session = %d %q", response.Code, response.Body.String())
	}
}

func (fixture JellyfinOwnerSessions) assertOwnerMediaPlayback(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	me := fixture.Call(t, handler, http.MethodGet, "/Users/Me", "", token)
	items := fixture.Call(t, handler, http.MethodGet, "/Items", "", token)
	var library struct {
		Items []struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
		} `json:"Items"`
	}
	fixture.Decode(t, items, &library)
	if me.Code != http.StatusOK || items.Code != http.StatusOK || len(library.Items) != 1 {
		t.Fatalf("media access: me=%d items=%d %q", me.Code, items.Code, items.Body.String())
	}
	playback := fixture.Call(t, handler, http.MethodPost, "/Items/"+library.Items[0].ID+"/PlaybackInfo", `{}`, token)
	stream := fixture.Call(t, handler, http.MethodGet, "/Items/"+library.Items[0].ID+"/File", "", token)
	if playback.Code != http.StatusOK || stream.Code != http.StatusOK {
		t.Fatalf("media playback: info=%d stream=%d", playback.Code, stream.Code)
	}
}

func (fixture JellyfinOwnerSessions) assertRestrictedOwnerRoutes(t *testing.T, handler http.Handler, token, label string) {
	t.Helper()
	for _, path := range []string{"/Users", "/Auth/Keys", "/api/v1/settings"} {
		if response := fixture.Call(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusForbidden {
			t.Fatalf("%s reached %s = %d %q", label, path, response.Code, response.Body.String())
		}
	}
}

// JellyfinCompatibilitySessionAppearsInActivityWithoutCredentials checks safe audit attribution.
func (fixture JellyfinOwnerSessions) JellyfinCompatibilitySessionAppearsInActivityWithoutCredentials(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler, ownerToken := fixture.APIServer(t)
	fixture.Enable(t, handler, ownerToken)
	fixture.DisableMFA(t, handler, ownerToken)
	created := APICall(t, handler, ownerToken, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Partner", "password": "partner-password", "owner": true})
	if created.Code != http.StatusCreated {
		t.Fatalf("Partner Owner = %d %q", created.Code, created.Body.String())
	}
	login := fixture.Call(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{"Username":"Partner","Pw":"partner-password"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("Jellyfin login = %d %q", login.Code, login.Body.String())
	}
	activity := APICall(t, handler, ownerToken, http.MethodGet, "/api/v1/activity?limit=100", nil)
	for _, expected := range []string{`"action":"session.created"`, `"actor":"Partner"`, `"channel":"jellyfin-compatibility"`, `"privileges":"media-only"`} {
		if !strings.Contains(activity.Body.String(), expected) {
			t.Fatalf("activity lacks %q: %s", expected, activity.Body.String())
		}
	}
	if strings.Contains(activity.Body.String(), "partner-password") {
		t.Fatalf("activity exposed a password: %s", activity.Body.String())
	}
}
