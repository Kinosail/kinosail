package server_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

// Only disposable fictional media is scanned. These bytes are not decode proof.
type q12PaginationFixture struct {
	web                  *httptest.Server
	client               *http.Client
	owner, grant, viewer string
}

type q12OriginTransport struct {
	origin *url.URL
	base   *http.Transport
}

func (transport q12OriginTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != transport.origin.Scheme || request.URL.Host != transport.origin.Host || request.URL.User != nil {
		return nil, errors.New("Q12 external peer denied")
	}
	return transport.base.RoundTrip(request)
}

func newQ12PaginationFixture(t *testing.T) *q12PaginationFixture {
	t.Helper()
	media := t.TempDir()
	root, err := os.OpenRoot(media)
	if err != nil {
		t.Fatal("Q12 temporary media root unavailable")
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error("Q12 fixture root close failed")
		}
	}()
	q12PopulateMedia(t, root)
	handler, owner := (servertest.APIFixture{
		NewHandler: func(_ string, data string) http.Handler {
			q12WriteLibrarySettings(t, data)
			return server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: data,
				CacheDir: t.TempDir(), RequireAuth: true, FFmpeg: "/q12-unavailable", FFprobe: "/q12-unavailable"})
		},
		TOTP: testTOTP,
	}).Server(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant",
		map[string]any{"enabled": true}), http.StatusOK)
	var offered struct{ Code string }
	offer := apiCall(t, handler, owner, http.MethodPost, "/api/v1/home-assistant/pairings", map[string]any{})
	assertAPIBody(t, offer, http.StatusCreated)
	mustJSON(t, offer, &offered)
	var grant struct{ Token string }
	paired := apiCall(t, handler, "", http.MethodPost, "/api/v1/home-assistant/pair",
		map[string]any{"code": offered.Code, "name": "Q12 fixture"})
	assertAPIBody(t, paired, http.StatusCreated)
	mustJSON(t, paired, &grant)
	if grant.Token == "" {
		t.Fatal("Q12 fixture grant missing")
	}
	created := apiCall(t, handler, owner, http.MethodPost, "/api/v1/profiles",
		map[string]any{"name": "Q12 Viewer", "password": "fictional-viewer-password", "rating": "all", "libraries": []string{"Visible"}})
	assertAPIBody(t, created, http.StatusCreated)
	var viewer struct{ Token string }
	login := apiCall(t, handler, "", http.MethodPost, "/api/v1/session",
		map[string]string{"name": "Q12 Viewer", "password": "fictional-viewer-password"})
	assertAPIBody(t, login, http.StatusCreated)
	mustJSON(t, login, &viewer)
	enrollTestAPIFactor(t, handler, viewer.Token)
	web, client := q12PaginationPeer(t, handler)
	return &q12PaginationFixture{web: web, client: client, owner: owner, grant: grant.Token, viewer: viewer.Token}
}

func q12PopulateMedia(t *testing.T, root *os.Root) {
	t.Helper()
	for _, group := range []struct {
		folder, prefix string
		count          int
	}{
		{"Visible", "Q12 Page", 201}, {"Private", "Q12 Private", 7},
	} {
		if err := root.MkdirAll(group.folder, 0o700); err != nil {
			t.Fatal("Q12 fixture directory unavailable")
		}
		for number := 1; number <= group.count; number++ {
			name := fmt.Sprintf("%s/%s %03d.mp4", group.folder, group.prefix, number)
			if err := root.WriteFile(name, []byte("fictional Q12 media"), 0o600); err != nil {
				t.Fatal("Q12 fixture item unavailable")
			}
		}
	}
}

func q12PaginationPeer(t *testing.T, handler http.Handler) (*httptest.Server, *http.Client) {
	t.Helper()
	web := httptest.NewServer(handler)
	t.Cleanup(web.Close)
	origin, err := url.Parse(web.URL)
	if err != nil || origin.Scheme != "http" || origin.User != nil || !net.ParseIP(origin.Hostname()).IsLoopback() {
		t.Fatal("Q12 loopback peer origin unavailable")
	}
	base := &http.Transport{Proxy: nil}
	client := &http.Client{Timeout: 10 * time.Second, Transport: q12OriginTransport{origin: origin, base: base},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(base.CloseIdleConnections)
	return web, client
}

func (fixture *q12PaginationFixture) get(t *testing.T, token, target string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fixture.web.URL+target, nil)
	if err != nil {
		t.Fatal("Q12 request construction failed")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := fixture.client.Do(request)
	if err != nil {
		t.Fatal("Q12 public peer unavailable")
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if errors.Join(readErr, response.Body.Close()) != nil || len(data) > 2<<20 {
		t.Fatal("Q12 response boundary failed")
	}
	return response.StatusCode, data
}

func q12PageTarget(route, query string, offset, limit int) string {
	values := url.Values{"limit": {fmt.Sprint(limit)}, "offset": {fmt.Sprint(offset)}}
	if query != "" {
		values.Set("q", query)
	}
	return route + "?" + values.Encode()
}

func q12WriteLibrarySettings(t *testing.T, data string) {
	t.Helper()
	root, err := os.OpenRoot(data)
	if err != nil {
		t.Fatal("Q12 temporary settings root unavailable")
	}
	writeErr := root.WriteFile("settings.json", []byte(`{"name":"Kinosail","libraries":["Visible","Private"],"updateChecks":false}`), 0o600)
	saved, readErr := root.ReadFile("settings.json")
	if errors.Join(writeErr, readErr, root.Close()) != nil {
		t.Fatal("Q12 explicit Library roots unavailable")
	}
	var settings struct {
		UpdateChecks *bool `json:"updateChecks"`
	}
	if json.Unmarshal(saved, &settings) != nil || settings.UpdateChecks == nil || *settings.UpdateChecks {
		t.Fatal("Q12 fixture external update checks not disabled before Server construction")
	}
}
