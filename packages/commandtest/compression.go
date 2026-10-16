package commandtest

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type compressionClient struct {
	server  *httptest.Server
	host    string
	session *http.Cookie
}

func HTTPCompression(t *testing.T, newHTTPServer func(context.Context) *http.Server) {
	t.Setenv("KINOSAIL_DATA_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	httpServer := newHTTPServer(ctx)
	client := compressionClient{server: httptest.NewServer(httpServer.Handler), host: httpServer.Addr}
	defer client.server.Close()
	client.session = ownerSession(t, httpServer)
	page := client.get(t, http.MethodGet, "/account", "same-origin", true, nil)
	html := decodedBody(t, page, "gzip")
	if !strings.Contains(html, `name="kinosail-csrf"`) || !strings.Contains(strings.Join(page.Header.Values("Vary"), ","), "Accept-Encoding") {
		t.Fatalf("same-origin page = %v %.120q", page.Header, html)
	}
	for _, site := range []string{"cross-site", "same-site", ""} {
		identity := decodedBody(t, client.get(t, http.MethodGet, "/account", site, true, nil), "")
		if !strings.Contains(identity, `name="kinosail-csrf"`) || len(identity) < 1024 {
			t.Fatalf("%q identity page = %.120q", site, identity)
		}
	}
	decodedBody(t, client.get(t, http.MethodGet, "/account", "same-origin", true, http.Header{"Range": {"bytes=0-99"}}), "")
	if head := client.get(t, http.MethodHead, "/account", "same-origin", true, nil); head.Header.Get("Content-Encoding") != "" {
		t.Fatalf("HEAD = %v", head.Header)
	}
	if login := decodedBody(t, client.get(t, http.MethodGet, "/login", "", false, nil), "gzip"); !strings.Contains(login, "<form") {
		t.Fatalf("public login = %.120q", login)
	}
	settings := client.get(t, http.MethodGet, "/api/v1/me", "same-origin", true, nil)
	if body := decodedBody(t, settings, "gzip"); !strings.HasPrefix(settings.Header.Get("Content-Type"), "application/json") || !strings.HasPrefix(body, "{") {
		t.Fatalf("profile JSON = %v %.80q", settings.Header, body)
	}
	plainStyle := decodedBody(t, client.get(t, http.MethodGet, "/static/app.css", "", false, http.Header{"Accept-Encoding": {"identity"}}), "")
	if style := decodedBody(t, client.get(t, http.MethodGet, "/static/app.css", "", true, nil), "gzip"); style != plainStyle {
		t.Fatal("compressed stylesheet differs from the identity stylesheet")
	}
	for _, signedIn := range []bool{false, true} {
		profile := client.get(t, http.MethodGet, "/debug/pprof/", "same-origin", signedIn, nil)
		if body := decodedBody(t, profile, profile.Header.Get("Content-Encoding")); profile.StatusCode == http.StatusOK || strings.Contains(body, "goroutine") {
			t.Fatalf("application listener served profiling: %d %.80q", profile.StatusCode, body)
		}
	}
}

func ownerSession(t *testing.T, httpServer *http.Server) *http.Cookie {
	t.Helper()
	setup := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader("name=Owner&password=owner-password"))
	setup.Host = httpServer.Addr
	setup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, setup)
	for _, cookie := range response.Result().Cookies() {
		if strings.HasSuffix(cookie.Name, "_session") && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatalf("setup = %d without a session cookie", response.Code)
	return nil
}

type observedResponse struct {
	StatusCode   int
	Header       http.Header
	method, path string
	body         []byte
}

func (client compressionClient) get(t *testing.T, method, path, site string, signedIn bool, extra http.Header) observedResponse {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, client.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = client.host
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	if site != "" {
		request.Header.Set("Sec-Fetch-Site", site)
	}
	if signedIn {
		request.AddCookie(client.session)
	}
	for name, values := range extra {
		request.Header[name] = values
	}
	transport := &http.Transport{DisableCompression: true}
	defer transport.CloseIdleConnections()
	response, err := (&http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return observedResponse{StatusCode: response.StatusCode, Header: response.Header, method: method, path: path, body: body}
}

func decodedBody(t *testing.T, response observedResponse, encoding string) string {
	t.Helper()
	if got := response.Header.Get("Content-Encoding"); got != encoding {
		t.Fatalf("%s %s Content-Encoding = %q, want %q (status %d %v)", response.method, response.path, got, encoding, response.StatusCode, response.Header)
	}
	if encoding != "gzip" {
		return string(response.body)
	}
	reader, err := gzip.NewReader(bytes.NewReader(response.body))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(decoded)
}
