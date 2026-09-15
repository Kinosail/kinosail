package jellyfincompat

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestPathRecognizesOnlyJellyfinPrefixes(t *testing.T) {
	for _, prefix := range jellyfinPrefixes {
		if !Path(prefix + "child") {
			t.Errorf("Jellyfin prefix rejected: %q", prefix)
		}
	}
	if Path("/api/v1/library") {
		t.Fatal("native API path was accepted")
	}
}

func TestCoreAndQuickConnectRoutesPreserveContract(t *testing.T) {
	mux := http.NewServeMux()
	called := map[string]int{}
	handler := func(name string) http.HandlerFunc {
		return func(http.ResponseWriter, *http.Request) { called[name]++ }
	}
	RegisterCore(mux, CoreHandlers{
		SystemInfo: handler("system"), Views: handler("views"), BitrateTest: handler("bitrate"), Authenticate: handler("password"),
		User: handler("user"), Users: handler("users"), CreateAuthKey: handler("create-key"), AuthKeys: handler("keys"), Logout: handler("logout"),
	})
	RegisterQuickConnect(mux, QuickConnectHandlers{Start: handler("start"), Status: handler("status"), Approve: handler("approve"), Authenticate: handler("quick-login")})
	for _, test := range []struct{ method, path, call string }{
		{http.MethodGet, "/System/Info/Public", "system"},
		{http.MethodGet, "/system/info/public", "system"},
		{http.MethodGet, "/System/Info", "system"},
		{http.MethodGet, "/Library/MediaFolders", "views"},
		{http.MethodGet, "/Playback/BitrateTest", "bitrate"},
		{http.MethodPost, "/Users/AuthenticateByName", "password"},
		{http.MethodGet, "/Users/Me", "user"},
		{http.MethodGet, "/Users/id", "user"},
		{http.MethodGet, "/Users", "users"},
		{http.MethodPost, "/Auth/Keys", "create-key"},
		{http.MethodGet, "/Auth/Keys", "keys"},
		{http.MethodPost, "/Sessions/Logout", "logout"},
		{http.MethodGet, "/QuickConnect/Initiate", "start"},
		{http.MethodPost, "/QuickConnect/Initiate", "start"},
		{http.MethodGet, "/QuickConnect/Connect", "status"},
		{http.MethodPost, "/QuickConnect/Authorize", "approve"},
		{http.MethodPost, "/Users/AuthenticateWithQuickConnect", "quick-login"},
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil))
		if response.Code != http.StatusOK || called[test.call] == 0 {
			t.Errorf("%s %s = %d, calls %v", test.method, test.path, response.Code, called)
		}
	}
	for _, path := range []string{"/Sessions/Capabilities", "/Sessions/Capabilities/Full"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))
		if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
			t.Errorf("POST %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	for path, want := range map[string]string{
		"/QuickConnect/Enabled": "true\n", "/Users/Public": "[]\n", "/Branding/Configuration": "{}\n",
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Body.String() != want {
			t.Errorf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
}

func TestEntryProjectionsPreservePlayerContract(t *testing.T) { //nolint:cyclop // One projection test protects the complete compatibility response.
	info := SystemInfo("server", "Kinosail", "https://media.test", true)
	wantInfo := map[string]any{"Id": "server", "ServerName": "Kinosail", "Version": Version, "ProductName": "Jellyfin Server", "OperatingSystem": "Linux", "StartupWizardCompleted": true, "LocalAddress": "https://media.test"}
	if !reflect.DeepEqual(info, wantInfo) {
		t.Fatalf("system info = %#v", info)
	}
	user := User{ID: "viewer", Name: "Sam", ServerID: "server", ServerName: "Kinosail", HasPassword: true, Downloads: true, Remote: true}
	dto := UserDTO(user)
	policy := dto["Policy"].(map[string]any)
	if dto["Id"] != "viewer" || policy["IsAdministrator"] != false || policy["EnableContentDownloading"] != true || policy["EnableRemoteAccess"] != true {
		t.Fatalf("Viewer DTO = %#v", dto)
	}
	owner := UserDTO(User{Owner: true, Disabled: true})["Policy"].(map[string]any)
	if owner["IsAdministrator"] != true || owner["IsDisabled"] != true || owner["EnableContentDownloading"] != true || owner["EnableRemoteAccess"] != true {
		t.Fatalf("owner policy = %#v", owner)
	}
	authentication := AuthenticationResult(Authentication{Token: "token", User: user})
	if authentication["AccessToken"] != "token" || authentication["ServerId"] != "server" || authentication["SessionInfo"].(map[string]any)["UserName"] != "Sam" {
		t.Fatalf("authentication = %#v", authentication)
	}
}

func TestJSONHelpersWriteExactUncachedResponses(t *testing.T) {
	response := httptest.NewRecorder()
	JSONStatus(response, map[string]bool{"ok": true}, http.StatusCreated)
	if response.Code != http.StatusCreated || response.Body.String() != "{\"ok\":true}\n" || response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("JSON response = %d %q %v", response.Code, response.Body.String(), response.Header())
	}
	response = httptest.NewRecorder()
	Value("fixed")(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Body.String() != "\"fixed\"\n" {
		t.Fatalf("fixed JSON = %q", response.Body.String())
	}
}

func TestPasswordAuthenticationPreservesValidationAndErrors(t *testing.T) { //nolint:cyclop,gocognit // One table proves every canonical HTTP translation.
	called := 0
	login := func(_ *http.Request, credentials Credentials) (Authentication, error) {
		called++
		if credentials.Username != "Sam" || credentials.Pw != "legacy" {
			t.Fatalf("credentials = %#v", credentials)
		}
		return Authentication{Token: "token", User: User{ID: "viewer", Name: "Sam", ServerID: "server"}}, nil
	}
	handler := PasswordAuthentication(func(*http.Request) bool { return false }, login)
	response := entryCall(t, handler, `{"Username":"Sam","Password":"legacy","Unknown":true}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"AccessToken":"token"`) || called != 1 {
		t.Fatalf("valid login = %d %q, calls %d", response.Code, response.Body.String(), called)
	}
	public := PasswordAuthentication(func(*http.Request) bool { return true }, login)
	response = entryCall(t, public, `{`)
	if response.Code != http.StatusForbidden || response.Body.String() != ErrPublicPasswordLogin.Error()+"\n" || called != 1 {
		t.Fatalf("public login = %d %q, calls %d", response.Code, response.Body.String(), called)
	}
	for name, body := range map[string]string{
		"malformed": `{`, "trailing": `{}` + `{}`, "long username": `{"Username":"` + strings.Repeat("u", 257) + `"}`,
		"long pw": `{"Pw":"` + strings.Repeat("p", 1025) + `"}`, "long password": `{"Password":"` + strings.Repeat("p", 1025) + `"}`,
		"oversized": strings.Repeat(" ", maxCredentialsBytes+1),
	} {
		response = entryCall(t, handler, body)
		if response.Code != http.StatusBadRequest || response.Body.String() != ErrInvalidCredentials.Error()+"\n" {
			t.Errorf("%s credentials = %d %q", name, response.Code, response.Body.String())
		}
	}
	nilBody := httptest.NewRecorder()
	handler(nilBody, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	if nilBody.Code != http.StatusBadRequest {
		t.Fatalf("nil body = %d", nilBody.Code)
	}
	for _, test := range []struct {
		err    error
		status int
		body   string
		retry  string
	}{
		{ErrInvalidCredentials, http.StatusUnauthorized, ErrInvalidCredentials.Error() + "\n", ""},
		{ErrCredentialRateLimit, http.StatusTooManyRequests, ErrCredentialRateLimit.Error() + "\n", "60"},
		{ErrQuickConnectMFA, http.StatusForbidden, ErrQuickConnectMFA.Error() + "\n", ""},
		{errors.New("database failed"), http.StatusInternalServerError, ErrCreateSession.Error() + "\n", ""},
	} {
		failing := PasswordAuthentication(func(*http.Request) bool { return false }, func(*http.Request, Credentials) (Authentication, error) { return Authentication{}, test.err })
		response = entryCall(t, failing, `{}`)
		if response.Code != test.status || response.Body.String() != test.body || response.Header().Get("Retry-After") != test.retry {
			t.Errorf("password error %v = %d %q retry %q", test.err, response.Code, response.Body.String(), response.Header().Get("Retry-After"))
		}
	}
}

func entryCall(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}
