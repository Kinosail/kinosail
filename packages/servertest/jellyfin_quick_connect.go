package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// JellyfinQuickConnectFixture binds the shared exchange scenarios to each real app.
type JellyfinQuickConnectFixture struct {
	NewHandler    func(*testing.T, time.Duration) http.Handler
	Remote        func(http.Handler) http.Handler
	SignIn        func(*testing.T, http.Handler, string, string) *http.Cookie
	AddViewer     func(*testing.T, http.Handler, *http.Cookie)
	Call          func(*testing.T, http.Handler, string, string, string, string) *httptest.ResponseRecorder
	Decode        func(*testing.T, *httptest.ResponseRecorder, any)
	WebCall       func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
	ConfirmFactor func(*testing.T, http.Handler, *http.Cookie, *httptest.ResponseRecorder)
}

// AssertJellyfinQuickConnect runs all five original real-handler regression scenarios.
func AssertJellyfinQuickConnect(t *testing.T, fixture JellyfinQuickConnectFixture) {
	t.Run("JellyfinAndroidTVQuickConnectUsesLocalViewerSession", func(t *testing.T) { assertJellyfinAndroidTVQuickConnectUsesLocalViewerSession(t, fixture) })
	t.Run("JellyfinQuickConnectRequiresDeviceIdentity", func(t *testing.T) { assertJellyfinQuickConnectRequiresDeviceIdentity(t, fixture) })
	t.Run("SwiftfinCanInitiateQuickConnectWithGET", func(t *testing.T) { assertSwiftfinCanInitiateQuickConnectWithGET(t, fixture) })
	t.Run("JellyfinPublicQuickConnectKeepsPublicViewerBoundary", func(t *testing.T) { assertJellyfinPublicQuickConnectKeepsPublicViewerBoundary(t, fixture) })
	t.Run("JellyfinQuickConnectRejectsOversizedDeviceIdentity", func(t *testing.T) { assertJellyfinQuickConnectRejectsOversizedDeviceIdentity(t, fixture) })
}

func assertJellyfinAndroidTVQuickConnectUsesLocalViewerSession(t *testing.T, fixture JellyfinQuickConnectFixture) {
	t.Parallel()
	handler := fixture.NewHandler(t, time.Minute)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	fixture.AddViewer(t, handler, owner)
	viewer := fixture.SignIn(t, handler, "/login", "name=Sam&password=viewer-password")
	state := initiateJellyfinQuickConnect(t, fixture, handler)
	pending := fixture.Call(t, handler, http.MethodGet, "/QuickConnect/Connect?secret="+state.Secret, "", "")
	if pending.Code != http.StatusOK || !strings.Contains(pending.Body.String(), `"Authenticated":false`) {
		t.Fatalf("Quick Connect pending = %d %q", pending.Code, pending.Body.String())
	}
	approve := fixture.WebCall(t, handler, http.MethodPost, "/QuickConnect/Authorize?Code="+state.Code, "", viewer)
	if approve.Code != http.StatusOK || strings.TrimSpace(approve.Body.String()) != "true" {
		t.Fatalf("Quick Connect approval = %d %q", approve.Code, approve.Body.String())
	}
	connected := fixture.Call(t, handler, http.MethodGet, "/QuickConnect/Connect?Secret="+state.Secret, "", "")
	if connected.Code != http.StatusOK || !strings.Contains(connected.Body.String(), `"Authenticated":true`) {
		t.Fatalf("Quick Connect connected = %d %q", connected.Code, connected.Body.String())
	}
	authenticateJellyfinQuickConnect(t, fixture, handler, state.Secret)
}

func assertJellyfinQuickConnectRequiresDeviceIdentity(t *testing.T, fixture JellyfinQuickConnectFixture) {
	t.Parallel()
	handler := fixture.NewHandler(t, 0)
	response := fixture.Call(t, handler, http.MethodPost, "/QuickConnect/Initiate", "", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing device identity = %d %q", response.Code, response.Body.String())
	}
}

func assertSwiftfinCanInitiateQuickConnectWithGET(t *testing.T, fixture JellyfinQuickConnectFixture) {
	t.Parallel()
	handler := fixture.NewHandler(t, 0)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Initiate", nil)
	request.Header.Set("Authorization", `MediaBrowser Client="Swiftfin iOS", Device="iPhone", DeviceId="phone-1", Version="1.3"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"Authenticated":false`) {
		t.Fatalf("Swiftfin Quick Connect initiate = %d %q", response.Code, response.Body.String())
	}
}

func assertJellyfinPublicQuickConnectKeepsPublicViewerBoundary(t *testing.T, fixture JellyfinQuickConnectFixture) {
	t.Parallel()
	handler := fixture.NewHandler(t, time.Minute)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	create := fixture.WebCall(t, handler, http.MethodPost, "/settings/profiles", "name=Viewer&password=viewer-password&rating=all&libraries=all&remote=true", owner)
	if create.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d %q", create.Code, create.Body.String())
	}
	viewer := fixture.SignIn(t, handler, "/login", "name=Viewer&password=viewer-password")
	enrollment := fixture.WebCall(t, handler, http.MethodPost, "/account/mfa/setup", "", viewer)
	fixture.ConfirmFactor(t, handler, viewer, enrollment)
	public := fixture.Remote(handler)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Initiate", nil)
	request.Header.Set("Authorization", `MediaBrowser Client="Swiftfin iOS", Device="iPhone", DeviceId="phone-1", Version="1.3"`)
	initiate := httptest.NewRecorder()
	public.ServeHTTP(initiate, request)
	var state struct{ Secret, Code string }
	fixture.Decode(t, initiate, &state)
	approved := fixture.WebCall(t, handler, http.MethodPost, "/QuickConnect/Authorize?Code="+state.Code, "", viewer)
	login := fixture.Call(t, public, http.MethodPost, "/Users/AuthenticateWithQuickConnect", `{"Secret":"`+state.Secret+`"}`, "")
	var session struct {
		Token string `json:"AccessToken"`
	}
	fixture.Decode(t, login, &session)
	items := fixture.Call(t, public, http.MethodGet, "/Items", "", session.Token)
	if initiate.Code != http.StatusOK || approved.Code != http.StatusOK || login.Code != http.StatusOK || session.Token == "" || items.Code != http.StatusOK {
		t.Fatalf("public Quick Connect: initiate=%d approve=%d login=%d items=%d", initiate.Code, approved.Code, login.Code, items.Code)
	}
}

func assertJellyfinQuickConnectRejectsOversizedDeviceIdentity(t *testing.T, fixture JellyfinQuickConnectFixture) {
	t.Parallel()
	handler := fixture.NewHandler(t, 0)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Initiate", nil)
	request.Header.Set("Authorization", `MediaBrowser Client="`+strings.Repeat("c", 81)+`", Device="TV", DeviceId="tv-1", Version="1"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized device identity = %d %q", response.Code, response.Body.String())
	}
}

type jellyfinQuickConnectState struct {
	Authenticated bool
	Secret, Code  string
}

func initiateJellyfinQuickConnect(t *testing.T, fixture JellyfinQuickConnectFixture, handler http.Handler) jellyfinQuickConnectState {
	t.Helper()
	enabled := fixture.Call(t, handler, http.MethodGet, "/QuickConnect/Enabled", "", "")
	if enabled.Code != http.StatusOK || strings.TrimSpace(enabled.Body.String()) != "true" {
		t.Fatalf("Quick Connect enabled = %d %q", enabled.Code, enabled.Body.String())
	}
	initiateRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Initiate", nil)
	initiateRequest.Header.Set("Authorization", `MediaBrowser Client="Jellyfin Android TV", Device="Bedroom TV", DeviceId="tv-1", Version="0.19.0"`)
	initiate := httptest.NewRecorder()
	handler.ServeHTTP(initiate, initiateRequest)
	var state jellyfinQuickConnectState
	fixture.Decode(t, initiate, &state)
	if initiate.Code != http.StatusOK || state.Authenticated || len(state.Secret) < 20 || len(state.Code) != 6 {
		t.Fatalf("Quick Connect initiate = %d %q", initiate.Code, initiate.Body.String())
	}
	return state
}

func authenticateJellyfinQuickConnect(t *testing.T, fixture JellyfinQuickConnectFixture, handler http.Handler, secret string) {
	t.Helper()
	login := fixture.Call(t, handler, http.MethodPost, "/Users/AuthenticateWithQuickConnect", `{"Secret":"`+secret+`"}`, "")
	var authenticated struct {
		AccessToken string
		User        struct{ Name string }
	}
	fixture.Decode(t, login, &authenticated)
	items := fixture.Call(t, handler, http.MethodGet, "/Items", "", authenticated.AccessToken)
	if login.Code != http.StatusOK || authenticated.User.Name != "Sam" || authenticated.AccessToken == "" || items.Code != http.StatusOK {
		t.Fatalf("Quick Connect login = %d %q, Items = %d %q", login.Code, login.Body.String(), items.Code, items.Body.String())
	}
	if reused := fixture.Call(t, handler, http.MethodPost, "/Users/AuthenticateWithQuickConnect", `{"Secret":"`+secret+`"}`, ""); reused.Code != http.StatusNotFound {
		t.Fatalf("reused Quick Connect secret = %d %q", reused.Code, reused.Body.String())
	}
}
