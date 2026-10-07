package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/catalog"
)

// The WebKit pagehide witness writes through the old resumed session after the
// explicit watched redirect. Both HTTP adapters must retain that transition.
func TestWatchedHTTPRejectsDepartingSessionProgress(t *testing.T) {
	for _, adapter := range []string{"web", "API"} {
		t.Run(adapter, func(t *testing.T) {
			fixture, before := preparedWatchedSessionHTTP(t, adapter)
			status := fixture.rejected(adapter)
			fixture.progress(t, adapter, "resumed-page", "2", "43", status)
			fixture.unchanged(t, before)
			fixture.progress(t, adapter, "resumed-page", "3", "0", status)
			fixture.unchanged(t, before)
			fixture.progress(t, adapter, "resumed-page", "0", "43", status)
			fixture.unchanged(t, before)
			fixture.progress(t, adapter, "new-user-playback", "1", "7", fixture.success(adapter))
			if after := fixture.state(t); after.Watched || after.Seconds != 7 || after.Session != "new-user-playback" {
				t.Fatal("new playback could not save progress after marking watched")
			}
		})
	}
}

func TestWatchedHTTPPreservesLegacyPlaybackAndExplicitUnwatched(t *testing.T) {
	for _, next := range []string{"session-less native", "explicit unwatched"} {
		t.Run(next, func(t *testing.T) {
			fixture := newWatchedSessionHTTP(t)
			fixture.progress(t, "API", "resumed-page", "1", "42", http.StatusOK)
			fixture.mark(t, true)
			session, revision := "", "0"
			if next == "explicit unwatched" {
				fixture.mark(t, false)
				session, revision = "resumed-page", "2"
			}
			fixture.progress(t, "API", session, revision, "9", http.StatusOK)
			if after := fixture.state(t); after.Watched || after.Seconds != 9 {
				t.Fatal("explicit unwatched or legacy playback stopped saving progress")
			}
		})
	}
}

type watchedSessionHTTP struct {
	endpoint *httptest.Server
	client   *http.Client
	cookie   *http.Cookie
	id       string
	csrf     string
	config   server.Config
}

func newWatchedSessionHTTP(t *testing.T) *watchedSessionHTTP {
	t.Helper()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := server.Config{MediaDir: media, DataDir: t.TempDir(), RequireAuth: true}
	handler := server.New(config)
	cookie := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	endpoint := httptest.NewServer(handler)
	t.Cleanup(endpoint.Close)
	client := endpoint.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	fixture := &watchedSessionHTTP{endpoint: endpoint, client: client, cookie: cookie, id: firstAPIItemID(t, handler, cookie.Value), config: config}
	page := fixture.request(t, http.MethodGet, "/", "", "", http.StatusOK)
	_, marker, found := strings.Cut(string(page), `<meta name="kinosail-csrf" content="`)
	if !found {
		t.Fatal("authenticated public page omitted CSRF metadata")
	}
	fixture.csrf, _, found = strings.Cut(marker, `"`)
	if !found || fixture.csrf == "" {
		t.Fatal("public CSRF metadata was empty")
	}
	return fixture
}

func preparedWatchedSessionHTTP(t *testing.T, adapter string) (*watchedSessionHTTP, catalog.PlaybackState) {
	t.Helper()
	fixture := newWatchedSessionHTTP(t)
	fixture.progress(t, "web", "resumed-page", "1", "42", http.StatusNoContent)
	fixture.mark(t, true)
	before := fixture.state(t)
	if !before.Watched || before.Seconds != 0 {
		t.Fatal("explicit watched transition was not committed")
	}
	if adapter == "API" {
		fixture.restart(t)
		fixture.unchanged(t, before)
	}
	return fixture, before
}

func (fixture *watchedSessionHTTP) unchanged(t *testing.T, before catalog.PlaybackState) {
	t.Helper()
	if fixture.state(t) != before {
		t.Fatal("departing session changed the committed watched state")
	}
}

func (fixture *watchedSessionHTTP) rejected(adapter string) int {
	if adapter == "API" {
		return http.StatusConflict
	}
	return http.StatusNoContent
}

func (fixture *watchedSessionHTTP) restart(t *testing.T) {
	t.Helper()
	fixture.endpoint.Close()
	fixture.endpoint = httptest.NewServer(server.New(fixture.config))
	t.Cleanup(fixture.endpoint.Close)
}

func (fixture *watchedSessionHTTP) request(t *testing.T, method, path, contentType, body string, status int) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, fixture.endpoint.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(fixture.cookie)
	request.Header.Set("Authorization", "Bearer "+fixture.cookie.Value)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Kinosail-CSRF", fixture.csrf)
	request.Header.Set("Origin", fixture.endpoint.URL)
	response, err := fixture.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode != status {
		t.Fatalf("public HTTP status=%d expected=%d read-error=%v", response.StatusCode, status, err)
	}
	return data
}

func (fixture *watchedSessionHTTP) progress(t *testing.T, adapter, session, revision, seconds string, status int) {
	t.Helper()
	values := url.Values{"seconds": {seconds}, "watched": {"false"}, "session": {session}, "revision": {revision}}
	if adapter == "web" {
		fixture.request(t, http.MethodPost, "/progress/"+fixture.id, "application/x-www-form-urlencoded", values.Encode(), status)
		return
	}
	// Literal numeric fields avoid deriving the oracle through production parsers.
	encodedSession, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"seconds":` + seconds + `,"watched":false,"session":` + string(encodedSession) + `,"revision":` + revision + `}`
	fixture.request(t, http.MethodPut, "/api/v1/items/"+fixture.id+"/progress", "application/json", body, status)
}

func (fixture *watchedSessionHTTP) mark(t *testing.T, watched bool) {
	t.Helper()
	body := "watched=false"
	if watched {
		body = "watched=true"
	}
	fixture.request(t, http.MethodPost, "/watched/"+fixture.id, "application/x-www-form-urlencoded", body, http.StatusSeeOther)
}

func (fixture *watchedSessionHTTP) state(t *testing.T) catalog.PlaybackState {
	t.Helper()
	data := fixture.request(t, http.MethodGet, "/api/v1/items/"+fixture.id, "", "", http.StatusOK)
	var document struct {
		Item struct{ Progress catalog.PlaybackState }
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document.Item.Progress
}

func (fixture *watchedSessionHTTP) success(adapter string) int {
	if adapter == "API" {
		return http.StatusOK
	}
	return http.StatusNoContent
}
