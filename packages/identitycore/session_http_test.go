package identitycore

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionTokenPrecedenceAndSources(t *testing.T) { //nolint:cyclop // The table proves the compatibility precedence contract.
	t.Parallel()
	query := func(*http.Request) (string, string) { return "query", "query-source" }
	cases := []struct {
		name, authorization, emby, media, api, browser, cookie, want, source string
		query                                                                QuerySessionToken
	}{
		{name: "bearer", authorization: "Bearer bearer", emby: "emby", want: "bearer", source: "authorization-bearer", query: query},
		{name: "empty bearer", authorization: "Bearer ", emby: "emby", want: "", source: "authorization-bearer", query: query},
		{name: "emby", emby: "emby", media: "media", want: "emby", source: "x-emby-token", query: query},
		{name: "media", media: "media", api: "api", want: "media", source: "x-mediabrowser-token", query: query},
		{name: "api", api: "api", browser: `MediaBrowser Token="browser"`, want: "api", source: "apikey-header", query: query},
		{name: "browser", browser: `MediaBrowser Client="x", Token="browser"`, want: "browser", source: "media-browser-authorization", query: query},
		{name: "query", cookie: "cookie", want: "query", source: "query-source", query: query},
		{name: "cookie", cookie: "cookie", want: "cookie", source: "kinosail-session-cookie"},
		{name: "none", source: "none"},
	}
	for _, test := range cases {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.Header.Set("Authorization", test.authorization)
		request.Header.Set("X-Emby-Token", test.emby)
		request.Header.Set("X-MediaBrowser-Token", test.media)
		request.Header.Set("ApiKey", test.api)
		request.Header.Set("X-Emby-Authorization", test.browser)
		if test.cookie != "" {
			request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: test.cookie, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
		}
		if got := SessionToken(request, test.query); got != test.want || SessionTokenSource(request, test.query) != test.source {
			t.Fatalf("%s = %q %q, want %q %q", test.name, got, SessionTokenSource(request, test.query), test.want, test.source)
		}
	}
	if SessionToken(nil, nil) != "" || SessionTokenSource(nil, nil) != "none" {
		t.Fatal("nil request did not fail closed")
	}
}

func TestMediaBrowserParsingAndSessionKeys(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("Authorization", `mediabrowser Device="TV", TOKEN="secret"`)
	if MediaBrowserValue(request, "token") != "secret" || MediaBrowserValue(request, "missing") != "" || MediaBrowserValue(nil, "token") != "" {
		t.Fatal("MediaBrowser parsing failed")
	}
	key := SessionKey("secret")
	if len(key) != 64 || SessionKeyIfNeeded(key) != key || SessionKeyIfNeeded("not-hex") != SessionKey("not-hex") || SessionKeyIfNeeded(strings.Repeat("z", 64)) != SessionKey(strings.Repeat("z", 64)) {
		t.Fatal("session key normalization failed")
	}
}

func TestCompatibilityLogPathAndContentRange(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Users/private.id/Items", nil)
	request.Header.Set("X-Emby-Authorization", `MediaBrowser Client="TV"`)
	if got := SafeCompatibilityPath(request, ""); got != "/Users/{value}/Items" {
		t.Fatalf("safe path = %q", got)
	}
	for _, test := range []struct {
		path, pattern, client string
	}{{strings.Repeat("/x", 257), "", "TV"}, {"/Users/1", "GET /Users/{id}", "TV"}, {"/Users/1", "", ""}, {strings.Repeat("/x", 13), "", "TV"}} {
		request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
		request.Header.Set("X-Emby-Authorization", `MediaBrowser Client="`+test.client+`"`)
		if SafeCompatibilityPath(request, test.pattern) != "" {
			t.Fatalf("unsafe path was accepted: %#v", test)
		}
	}
	for value, want := range map[string]bool{"bytes 0-0/1": true, "bytes */1": true, "bytes 1-0/2": false, "bytes 0-2/2": false, "bad": false, strings.Repeat("x", 129): false} {
		if got := ValidContentRange(value); got != want {
			t.Errorf("ValidContentRange(%q) = %t, want %t", value, got, want)
		}
	}
}

func TestSessionNormalizationNamesAndCookies(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	assertSessionNormalization(t, now)
	assertAutomaticSessionTimestamp(t)
	assertCleanDeviceNames(t)
	assertSecureSessionCookies(t, now)
}

func assertSessionNormalization(t *testing.T, now time.Time) {
	t.Helper()
	hashed := SessionKey("hashed")
	values := map[string]Session{"raw": {}, hashed: {Name: "Known", CreatedAt: 1}}
	NormalizeSessions(values, func() time.Time { return now })
	if len(values) != 2 || values[SessionKey("raw")].Name != "Legacy device" || values[SessionKey("raw")].CreatedAt != now.Unix() || values[hashed].Name != "Known" || values[hashed].CreatedAt != 1 {
		t.Fatalf("normalized = %#v", values)
	}
}

func assertAutomaticSessionTimestamp(t *testing.T) {
	t.Helper()
	auto := map[string]Session{"raw": {}}
	started := time.Now().Unix()
	NormalizeSessions(auto, nil)
	if auto[SessionKey("raw")].CreatedAt < started {
		t.Fatalf("automatic timestamp = %#v", auto)
	}
}

func assertCleanDeviceNames(t *testing.T) {
	t.Helper()
	for name, want := range map[string]string{
		"": "Web browser", " Mozilla Firefox/1 ": "Firefox", "Mozilla Edg/1 Chrome/1 Safari/1": "Microsoft Edge",
		"Mozilla Chrome/1 Safari/1": "Chrome", "Mozilla Safari/1": "Safari", "TV": "TV",
	} {
		if got := CleanDeviceName(name); got != want {
			t.Fatalf("clean %q = %q, want %q", name, got, want)
		}
	}
	long := strings.Repeat("x", 81)
	if len(CleanDeviceName(long)) != 80 || CleanDeviceName(strings.Repeat("x", 80)) != strings.Repeat("x", 80) {
		t.Fatal("device name bound failed")
	}
}

func assertSecureSessionCookies(t *testing.T, now time.Time) {
	t.Helper()
	standard, public := SessionCookie("token"), PublicSessionCookie("token", now)
	assertStandardSessionCookie(t, standard)
	assertPublicSessionCookie(t, public, now)
}

func assertStandardSessionCookie(t *testing.T, standard *http.Cookie) {
	t.Helper()
	if standard.Name != "__Host-kinosail_session" || standard.Value != "token" || standard.Path != "/" || !standard.HttpOnly || !standard.Secure || standard.SameSite != http.SameSiteStrictMode || standard.MaxAge != 0 {
		t.Fatalf("session cookie = %#v", standard)
	}
}

func assertPublicSessionCookie(t *testing.T, public *http.Cookie, now time.Time) {
	t.Helper()
	if public.MaxAge != 8*60*60 || !public.Expires.Equal(now.Add(8*time.Hour)) || !public.HttpOnly || !public.Secure || public.SameSite != http.SameSiteStrictMode {
		t.Fatalf("public cookie = %#v", public)
	}
}
