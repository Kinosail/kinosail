package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type shelfClient struct {
	t       *testing.T
	handler http.Handler
	ids     map[string]string
}

func (client shelfClient) post(cookie *http.Cookie, path, body string) {
	client.t.Helper()
	request := httptest.NewRequestWithContext(client.t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	client.handler.ServeHTTP(response, request)
	if response.Code >= http.StatusBadRequest {
		client.t.Fatalf("POST %s = %d %q", path, response.Code, response.Body.String())
	}
}

func (client shelfClient) home(cookie *http.Cookie) string {
	client.t.Helper()
	request := httptest.NewRequestWithContext(client.t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	client.handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		client.t.Fatalf("GET / = %d", response.Code)
	}
	return response.Body.String()
}

func homeShelfTitles(body, heading string) []string {
	section := regexp.MustCompile(`(?s)<section class="home-shelf[^"]*"><header><h2>` + heading + `</h2>.*?</section>`).FindString(body)
	titles := make([]string, 0)
	for _, match := range regexp.MustCompile(`(?s)<a class="(?:resume-link|card)[^"]*" href="/watch/([a-f0-9]+)"`).FindAllStringSubmatch(section, -1) {
		titles = append(titles, match[1])
	}
	return titles
}

func homeDestinationCount(body, name string) string {
	match := regexp.MustCompile(`<strong>` + name + `</strong><small>[^<]*</small></span><small>([^<]*)</small>`).FindStringSubmatch(body)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func TestHomeLandingShelvesFollowEachViewersStateAndVisibleTitles(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	titles := map[string]string{"Arrival": "PG", "Brazil": "PG", "Heat": "PG", "Memento": "R", "Zodiac": "PG"}
	for title, rating := range titles {
		for name, content := range map[string]string{title + ".mp4": "video", title + ".nfo": "<movie><title>" + title + "</title><mpaa>" + rating + "</mpaa></movie>"} {
			if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	client := shelfClient{t: t, handler: handler, ids: make(map[string]string)}
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	client.post(owner, "/settings/profiles", "name=Kid&password=viewer-password&rating=all&libraries=all")
	kid := signInTestProfile(t, handler, "/login", "name=Kid&password=viewer-password")
	for title := range titles {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?q="+title, nil)
		request.AddCookie(owner)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		match := regexp.MustCompile(`href="/watch/([a-f0-9]+)"`).FindStringSubmatch(response.Body.String())
		if len(match) != 2 {
			t.Fatalf("%s was not found", title)
		}
		client.ids[title] = match[1]
	}
	ids := func(names ...string) []string {
		result := make([]string, 0, len(names))
		for _, name := range names {
			result = append(result, client.ids[name])
		}
		return result
	}
	for _, title := range []string{"Zodiac", "Memento", "Arrival"} {
		client.post(owner, "/progress/"+client.ids[title], "seconds=60")
	}
	client.post(owner, "/watched/"+client.ids["Heat"], "watched=true")
	client.post(owner, "/list/"+client.ids["Zodiac"], "listed=true")
	client.post(owner, "/list/"+client.ids["Brazil"], "listed=true")
	client.post(kid, "/progress/"+client.ids["Memento"], "seconds=90")
	client.post(kid, "/progress/"+client.ids["Brazil"], "seconds=30")
	client.post(kid, "/list/"+client.ids["Memento"], "listed=true")
	client.post(kid, "/list/"+client.ids["Arrival"], "listed=true")
	client.post(owner, "/settings/profiles/permissions", "id="+storedProfileID(t, dataDir, "Kid")+"&rating=family&libraries=all")

	ownerHome, kidHome := client.home(owner), client.home(kid)
	for _, check := range []struct {
		viewer, shelf string
		body          string
		want          []string
	}{
		{"owner", "Continue watching", ownerHome, ids("Arrival", "Memento", "Zodiac")},
		{"owner", "My List", ownerHome, ids("Brazil", "Zodiac")},
		{"owner", "Recently played", ownerHome, ids("Heat", "Arrival", "Memento", "Zodiac")},
		{"kid", "Continue watching", kidHome, ids("Brazil")},
		{"kid", "My List", kidHome, ids("Arrival")},
		{"kid", "Recently played", kidHome, ids("Brazil")},
	} {
		if got := homeShelfTitles(check.body, check.shelf); !reflect.DeepEqual(got, check.want) {
			t.Errorf("%s %s = %q, want %q", check.viewer, check.shelf, got, check.want)
		}
	}
	for _, check := range []struct{ viewer, body, list string }{{"owner", ownerHome, "2 items"}, {"kid", kidHome, "1 item"}} {
		if got := homeDestinationCount(check.body, "My List"); got != check.list {
			t.Errorf("%s My List destination = %q, want %q", check.viewer, got, check.list)
		}
	}
	if strings.Contains(kidHome, client.ids["Memento"]) {
		t.Fatal("restricted landing exposed a hidden title")
	}
}
