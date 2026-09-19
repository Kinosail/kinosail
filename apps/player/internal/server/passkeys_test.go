package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/go-webauthn/webauthn/webauthn"
)

//nolint:cyclop // The integration assertion checks all security attributes of the one ceremony response.
func TestPasskeyCeremoniesAreLocalAndServerSide(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "http://localhost:8080"})
	owner := servertest.SetupOwnerCookie(t, handler)

	account := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account", nil)
	request.Host = "localhost:8080"
	request.AddCookie(owner)
	handler.ServeHTTP(account, request)
	if account.Code != http.StatusOK || !strings.Contains(account.Body.String(), "Add a passkey") || !strings.Contains(account.Body.String(), "Add one directly on another device") || !strings.Contains(account.Body.String(), "/static/app.css?v=electric-13") || !strings.Contains(account.Body.String(), "/static/passkeys.js?v=13") {
		t.Fatalf("account = %d %q", account.Code, account.Body.String())
	}
	prompt := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account?setup=1", nil)
	request.Host = "localhost:8080"
	request.AddCookie(owner)
	handler.ServeHTTP(prompt, request)
	if prompt.Code != http.StatusOK || !strings.Contains(prompt.Body.String(), "Protect the Owner account") || strings.Contains(prompt.Body.String(), "Not now") {
		t.Fatalf("prompt = %d %q", prompt.Code, prompt.Body.String())
	}

	begin := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/register/begin", nil)
	request.Host = "localhost:8080"
	request.AddCookie(owner)
	handler.ServeHTTP(begin, request)
	cookies := begin.Result().Cookies()
	if begin.Code != http.StatusOK || !strings.Contains(begin.Body.String(), `"rp":{"name":"Kinosail","id":"localhost"}`) || !strings.Contains(begin.Body.String(), `"userVerification":"required"`) || len(cookies) != 1 {
		t.Fatalf("begin = %d, cookies = %#v, body = %q", begin.Code, cookies, begin.Body.String())
	}
	if cookies[0].Name != "kinosail_passkey" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("passkey cookie = %#v", cookies[0])
	}
	if cookies[0].Path != "/api/v1/passkeys/" {
		t.Fatalf("passkey cookie path = %q", cookies[0].Path)
	}
	if strings.Contains(cookies[0].Value, "challenge") {
		t.Fatal("passkey challenge was exposed in the cookie")
	}
	finish := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/register/finish", strings.NewReader(`{}`))
	request.Host = "localhost:8080"
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(owner)
	request.AddCookie(cookies[0])
	handler.ServeHTTP(finish, request)
	if finish.Code != http.StatusBadRequest || strings.Contains(finish.Body.String(), "expired") {
		t.Fatalf("finish = %d %q", finish.Code, finish.Body.String())
	}
}

func TestPasskeyLoginBeginIsPublic(t *testing.T) {
	t.Parallel()
	servertest.PasskeyLoginBeginIsPublic(t, New(Config{DataDir: t.TempDir(), RequireAuth: true}), "https://localhost:38127")
}

func TestPasskeyLoginIgnoresStaleSameOriginSessionCookie(t *testing.T) {
	t.Parallel()
	const origin = "https://localhost:38127"
	servertest.PasskeyLoginIgnoresStaleSession(t, New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: origin}), origin)
}

func TestPasskeyBeginRedirectsToConfiguredOrigin(t *testing.T) {
	t.Parallel()
	configured, err := configuration.Load(t.TempDir(), "", func(name string) (string, bool) {
		return `["192.0.2.55"]`, name == "KINOSAIL_TLS_HOSTS"
	})
	if err != nil {
		t.Fatal(err)
	}
	servertest.PasskeyBeginRedirectsToConfiguredOrigin(t, New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://media.example:38127", Configuration: configured}), "https://media.example:38127", "https://192.0.2.55:38127")
}

func TestPasskeyLoginIsAvailableFromSignInPage(t *testing.T) {
	t.Parallel()
	servertest.PasskeyLoginIsAvailableFromSignInPage(t, New(Config{DataDir: t.TempDir(), RequireAuth: true}), "/static/passkeys.js?v=13")
}

func TestPasswordLoginOffersPasskeyWithConfiguredStateAndSafeReturn(t *testing.T) { //nolint:cyclop,funlen,gocognit // One login boundary covers both inventory states and invalid offer input.
	t.Parallel()
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "not configured", true: "configured"}[configured], func(t *testing.T) {
			dataDir := t.TempDir()
			store := newProfileStore(dataDir)
			profile, _ := newProfile("Viewer", "viewer-password", false)
			if err := addTestOwner(store, profile); err != nil {
				t.Fatal(err)
			}
			if err := configuration.Set(dataDir, "security.require_mfa", "false"); err != nil {
				t.Fatal(err)
			}
			configuredSettings, err := configuration.Load(dataDir, "", func(string) (string, bool) { return "", false })
			if err != nil {
				t.Fatal(err)
			}
			if configured {
				if err := store.addPasskey(profile.ID, &webauthn.Credential{ID: []byte("existing-passkey"), PublicKey: []byte("key")}); err != nil {
					t.Fatal(err)
				}
			}
			handler := New(Config{DataDir: dataDir, RequireAuth: true, Configuration: configuredSettings, AuthURL: "http://localhost:8080"})
			form := url.Values{"name": {"Viewer"}, "password": {"viewer-password"}}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login?next=%2F%3Fview%3Dmovies", strings.NewReader(form.Encode()))
			request.Host = "localhost:8080"
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			login := httptest.NewRecorder()
			handler.ServeHTTP(login, request)
			if login.Code != http.StatusSeeOther || login.Header().Get("Location") != "/account?passkey=offer&next=%2F%3Fview%3Dmovies" {
				t.Fatalf("login = %d, location = %q", login.Code, login.Header().Get("Location"))
			}
			cookie := login.Result().Cookies()[0]
			offer := passkeyRequestWithCookie(t, handler, login.Header().Get("Location"), cookie)
			want := "No passkey is saved for this account yet"
			if configured {
				want = "A passkey is already set up"
			}
			if offer.Code != http.StatusOK || !strings.Contains(offer.Body.String(), want) || !strings.Contains(offer.Body.String(), `href="/?view=movies"`) || !strings.Contains(offer.Body.String(), "http://localhost:8080/account") {
				t.Fatalf("offer = %d %q", offer.Code, offer.Body.String())
			}
			apiLogin := httptest.NewRecorder()
			apiRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/session", strings.NewReader(form.Encode()))
			apiRequest.Host = "localhost:8080"
			apiRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handler.ServeHTTP(apiLogin, apiRequest)
			if apiLogin.Code != http.StatusCreated || !strings.Contains(apiLogin.Body.String(), `"configured":`+map[bool]string{false: "false", true: "true"}[configured]) || !strings.Contains(apiLogin.Body.String(), `"usedForSignIn":false`) {
				t.Fatalf("API login = %d %q", apiLogin.Code, apiLogin.Body.String())
			}

			before := passkeyRequestWithCookie(t, handler, "/api/v1/passkeys", cookie).Body.String()
			for _, path := range []string{"/account?passkey=unknown", "/account?passkey=offer&passkey=offer", "/account?passkey=offer&next=https%3A%2F%2Foutside.example", "/account?passkey=offer&next=" + url.QueryEscape(strings.Repeat("x", 2049))} {
				if response := passkeyRequestWithCookie(t, handler, path, cookie); response.Code != http.StatusBadRequest {
					t.Errorf("GET %s = %d %q", path, response.Code, response.Body.String())
				}
			}
			if after := passkeyRequestWithCookie(t, handler, "/api/v1/passkeys", cookie).Body.String(); after != before {
				t.Fatalf("invalid offers changed inventory: before=%q after=%q", before, after)
			}
			stepUp := httptest.NewRecorder()
			stepUpRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login?stepup=1&next=%2Fsettings%2Fbackups", strings.NewReader(form.Encode()))
			stepUpRequest.Host = "localhost:8080"
			stepUpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handler.ServeHTTP(stepUp, stepUpRequest)
			if stepUp.Code != http.StatusSeeOther || stepUp.Header().Get("Location") != "/settings/backups" {
				t.Fatalf("step-up = %d, location = %q", stepUp.Code, stepUp.Header().Get("Location"))
			}
		})
	}
}

func TestPasskeyLoginBeginUsesLoginRateLimit(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true})
	var response *httptest.ResponseRecorder
	for range 11 {
		response = httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/begin", nil)
		request.Host = "localhost:38127"
		request.RemoteAddr = "192.0.2.1:1234"
		handler.ServeHTTP(response, request)
	}
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("rate limit = %d, retry = %q", response.Code, response.Header().Get("Retry-After"))
	}
}

func TestInvalidAuthURLDisablesOnlyPasskeys(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://example.com/not-an-origin"})
	if owner := servertest.SetupOwnerCookie(t, handler); owner == nil {
		t.Fatal("password setup failed")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/begin", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("passkey status = %d", response.Code)
	}
}

func TestPasskeyCredentialPersistsAndUpdates(t *testing.T) { //nolint:cyclop // One persistence test covers credential and usage metadata together.
	t.Parallel()
	dataDir := t.TempDir()
	store := newProfileStore(dataDir)
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil || addTestOwner(store, profile) != nil {
		t.Fatal(err)
	}
	credential := webauthn.Credential{ID: []byte("credential"), PublicKey: []byte("public-key")}
	if err := store.addPasskey(profile.ID, &credential); err != nil {
		t.Fatal(err)
	}
	credential.Authenticator.SignCount = 7
	if err := store.updatePasskey(profile.ID, &credential); err != nil {
		t.Fatal(err)
	}
	reloaded := newProfileStore(dataDir)
	if reloaded.err != nil || len(reloaded.profiles) != 1 || len(reloaded.profiles[0].Passkeys) != 1 || reloaded.profiles[0].Passkeys[0].Authenticator.SignCount != 7 || !reloaded.profiles[0].PasskeyUsage[sharedpasskeys.ID(credential.ID)].Tracked || reloaded.profiles[0].PasskeyUsage[sharedpasskeys.ID(credential.ID)].LastUsed == 0 {
		t.Fatalf("reloaded profiles = %#v, err = %v", reloaded.profiles, reloaded.err)
	}
}

func passkeyRequestWithCookie(t *testing.T, handler http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.Host = "localhost:8080"
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
