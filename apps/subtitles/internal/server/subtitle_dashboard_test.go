package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleAppShowsCoverageAndWantedFiles(t *testing.T) { //nolint:cyclop // One fixture verifies the three related dashboard projections.
	t.Parallel()
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival (2016).mp4"), "video")
	writeTestFile(t, filepath.Join(media, "Severance", "S01", "Severance.S01E01.mkv"), "video")
	writeTestFile(t, filepath.Join(media, "Severance", "S01", "Severance.S01E01.en.srt"), "1\n00:00:01,000 --> 00:00:02,000\nHello\n")
	writeTestFile(t, filepath.Join(media, "soundtrack.flac"), "audio")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})

	overview := requestApp(t, handler, http.MethodGet, "/", "")
	wanted := requestApp(t, handler, http.MethodGet, "/?view=wanted", "")
	library := requestApp(t, handler, http.MethodGet, "/?view=library", "")
	titleSearch := requestApp(t, handler, http.MethodGet, "/?view=library&q=arrival", "")
	showSearch := requestApp(t, handler, http.MethodGet, "/?view=library&q=severance", "")
	fileSearch := requestApp(t, handler, http.MethodGet, "/?view=library&q=s01e01.mkv", "")

	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), "50%") || !strings.Contains(overview.Body.String(), "file needs") || !strings.Contains(overview.Body.String(), "Arrival") || strings.Contains(overview.Body.String(), "soundtrack") {
		t.Fatalf("overview = %d %q", overview.Code, overview.Body.String())
	}
	for _, expected := range []string{`class="library-page subtitle-app subtitle-dashboard"`, `class="app-header"`, `class="brand-lockup"`, `href="/?view=summary"`, `aria-current="page"`, `/static/app.css?v=electric-1`} {
		if !strings.Contains(overview.Body.String(), expected) {
			t.Fatalf("overview shell missing %q: %q", expected, overview.Body.String())
		}
	}
	if strings.Contains(overview.Body.String(), `class="subtitle-sidebar"`) || strings.Contains(overview.Body.String(), `class="subtitle-mobile-nav"`) {
		t.Fatalf("overview still uses the legacy subtitle shell: %q", overview.Body.String())
	}
	if wanted.Code != http.StatusOK || !strings.Contains(wanted.Body.String(), "Arrival") || strings.Contains(wanted.Body.String(), "Severance.S01E01.mkv") {
		t.Fatalf("wanted = %d %q", wanted.Code, wanted.Body.String())
	}
	if library.Code != http.StatusOK || !strings.Contains(library.Body.String(), "Arrival") || !strings.Contains(library.Body.String(), "Severance.S01E01.mkv") || !strings.Contains(library.Body.String(), ">en<") {
		t.Fatalf("library = %d %q", library.Code, library.Body.String())
	}
	for name, response := range map[string]*httptest.ResponseRecorder{"title": titleSearch, "show": showSearch, "file": fileSearch} {
		if response.Code != http.StatusOK || strings.Count(response.Body.String(), `class="subtitle-file"`) != 1 {
			t.Fatalf("%s search = %d %q", name, response.Code, response.Body.String())
		}
	}
}

func TestSubtitleDashboardRejectsAmbiguousAndOversizedQueries(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: t.TempDir(), CacheDir: t.TempDir()})
	for name, path := range map[string]string{
		"unknown":         "/?unknown=true",
		"duplicate":       "/?view=wanted&view=library",
		"view":            "/?view=players",
		"oversized":       "/?q=" + strings.Repeat("x", 129),
		"query array":     "/?q=one&q=two",
		"api unknown":     "/api/v1/subtitle-library?unknown=true",
		"api unused lang": "/api/v1/subtitle-library?lang=fr",
	} {
		t.Run(name, func(t *testing.T) {
			response := requestApp(t, handler, http.MethodGet, path, "")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestBrowserErrorsRenderBrandedShellAndAPIErrorsRemainJSON(t *testing.T) { //nolint:cyclop // Browser and API error contracts are checked together.
	t.Parallel()
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Needs Subtitles.mp4"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	invalid := requestApp(t, handler, http.MethodGet, "/?view=wanted&view=library", "")
	if invalid.Code != http.StatusBadRequest || !strings.HasPrefix(invalid.Header().Get("Content-Type"), "text/html") || !strings.Contains(invalid.Body.String(), "Request could not be completed.") || !strings.Contains(invalid.Body.String(), "Return to Subtitles") {
		t.Fatalf("invalid browser request = %d %q", invalid.Code, invalid.Body.String())
	}
	overview := requestApp(t, handler, http.MethodGet, "/", "")
	if !strings.Contains(overview.Body.String(), `href="/settings#provider">Connect a subtitle source`) || !strings.Contains(overview.Body.String(), `id="subtitle-maintain-help"`) {
		t.Fatalf("subtitle source setup lacks its explanation: %q", overview.Body.String())
	}

	missing := requestApp(t, handler, http.MethodGet, "/missing-page", "")
	if missing.Code != http.StatusNotFound || !strings.HasPrefix(missing.Header().Get("Content-Type"), "text/html") || !strings.Contains(missing.Body.String(), "Page not found") || !strings.Contains(missing.Body.String(), "That Subtitles page does not exist") {
		t.Fatalf("missing browser page = %d %q", missing.Code, missing.Body.String())
	}

	api := requestApp(t, handler, http.MethodGet, "/api/v1/not-a-route", "")
	if api.Code != http.StatusNotFound || !strings.HasPrefix(api.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("missing API route = %d %q", api.Code, api.Body.String())
	}
}

func TestSubtitleDashboardRequiresLocalTrackForUnsupportedLanguage(t *testing.T) { //nolint:cyclop // The test covers local/provider language fallback boundaries.
	t.Parallel()
	var searches atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		searches.Add(1)
		http.Error(writer, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	if settings := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["aa"]}`); settings.Code != http.StatusOK {
		t.Fatalf("settings = %d %q", settings.Code, settings.Body.String())
	}
	var inventory struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library", "")
	if json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 || !strings.Contains(response.Body.String(), `"providerAvailable":false`) || !strings.Contains(response.Body.String(), `"searchable":false`) {
		t.Fatalf("inventory = %d %q", response.Code, response.Body.String())
	}
	page := requestApp(t, handler, http.MethodGet, "/", "")
	player := requestApp(t, handler, http.MethodGet, "/watch/"+inventory.Items[0].ID, "")
	web := requestApp(t, handler, http.MethodPost, "/subtitles/manage/"+inventory.Items[0].ID+"/fetch", "")
	maintenance := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/maintain", `{"limit":1}`)
	if !strings.Contains(page.Body.String(), "Add a local subtitle") || strings.Contains(page.Body.String(), "Find aa") || strings.Contains(player.Body.String(), "Find aa") || web.Code != http.StatusConflict || maintenance.Code != http.StatusOK || !strings.Contains(maintenance.Body.String(), `"attempted":0`) || searches.Load() != 0 {
		t.Fatalf("page=%q player=%q web=%d maintenance=%d %q searches=%d", page.Body.String(), player.Body.String(), web.Code, maintenance.Code, maintenance.Body.String(), searches.Load())
	}
}

func TestSubtitleCoverageClassifiesDefaultOtherAndDoubleDigitEpisodes(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Default.mp4"), "video")
	writeTestFile(t, filepath.Join(media, "Default.srt"), "subtitle")
	writeTestFile(t, filepath.Join(media, "Other.mp4"), "video")
	writeTestFile(t, filepath.Join(media, "Other.commentary.srt"), "subtitle")
	writeTestFile(t, filepath.Join(media, "Example", "S12", "Example.S12E11.mkv"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})

	library := requestApp(t, handler, http.MethodGet, "/?view=library", "")
	if library.Code != http.StatusOK || !strings.Contains(library.Body.String(), ">Default<") || !strings.Contains(library.Body.String(), ">Other<") || !strings.Contains(library.Body.String(), "S12E11") {
		t.Fatalf("library = %d %q", library.Code, library.Body.String())
	}
}

func TestSubtitleSettingsAPIRendersEverySavedLanguageAtTheLimit(t *testing.T) {
	t.Parallel()
	languages := []string{"es-419", "pt-MZ", "zh-Hant", "sr-Cyrl", "yue", "ckb", "mni", "cnr", "sat", "syr", "tet", "tok", "azb", "ast", "ext", "prs", "fil", "aa", "en", "de"}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: t.TempDir(), CacheDir: t.TempDir()})
	body, err := json.Marshal(map[string]any{"languages": languages})
	if err != nil {
		t.Fatal(err)
	}
	if saved := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", string(body)); saved.Code != http.StatusOK {
		t.Fatalf("save = %d %q", saved.Code, saved.Body.String())
	}
	api := requestApp(t, handler, http.MethodGet, "/api/v1/settings", "")
	page := requestApp(t, handler, http.MethodGet, "/settings", "")
	start := strings.Index(page.Body.String(), `<ol class="subtitle-language-list"`)
	end := strings.Index(page.Body.String(), `</ol>`)
	rows := 0
	if start >= 0 && end > start {
		rows = strings.Count(page.Body.String()[start:end], "<li>")
	}
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"subtitleLanguages":["es-419","pt-MZ","zh-Hant"`) || page.Code != http.StatusOK || rows != len(languages) || !strings.Contains(page.Body.String(), "The 20-language limit is reached") {
		t.Fatalf("api = %d %q, page = %d rows=%d", api.Code, api.Body.String(), page.Code, rows)
	}
}

func TestSubtitleAppUsesKinosailSisterSetupAndFocusedSettings(t *testing.T) { //nolint:cyclop // Sister-app setup, settings, and onboarding stay one visible experience contract.
	t.Parallel()
	media := t.TempDir()
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	setup := requestApp(t, handler, http.MethodGet, "/setup", "")
	settings := requestApp(t, handler, http.MethodGet, "/settings", "")
	start := requestApp(t, handler, http.MethodGet, "/onboarding", "")
	onboarding := requestApp(t, handler, http.MethodGet, "/onboarding/connection", "")
	setupBody := setup.Body.String()
	if setup.Code != http.StatusOK || !strings.Contains(setupBody, "Kinosail Subtitles") || !strings.Contains(setupBody, "your subtitle settings") || !strings.Contains(setupBody, "saved subtitle files") || !strings.Contains(setupBody, `/static/app.css?v=electric-1`) || !strings.Contains(setupBody, `class="language-picker"`) || strings.Index(setupBody, `class="language-picker"`) > strings.Index(setupBody, `class="wizard-stage"`) {
		t.Fatalf("setup = %d %q", setup.Code, setup.Body.String())
	}
	if settings.Code != http.StatusOK {
		t.Fatalf("settings = %d %q", settings.Code, settings.Body.String())
	}
	assertResponseContains(t, "settings", settings, "Subtitle settings", "Preferred languages", `aria-label="Preferred subtitle languages"`, "Add a language", `value="es-419"`, "Media Libraries", `href="#appearance">Appearance`, `<legend>Theme</legend>`, `value="dark" data-theme-choice checked`, "Trusted HTTPS", "myhome-subtitles.duckdns.org", `action="/settings/trusted-https"`, "Open setup guide")
	if strings.Contains(settings.Body.String(), "Transcoder") {
		t.Fatalf("settings unexpectedly expose transcoder controls: %q", settings.Body.String())
	}
	if start.Code != http.StatusSeeOther || start.Header().Get("Location") != "/onboarding/connection" {
		t.Fatalf("onboarding start = %d %q", start.Code, start.Header().Get("Location"))
	}
	if onboarding.Code != http.StatusOK {
		t.Fatalf("onboarding = %d %q", onboarding.Code, onboarding.Body.String())
	}
	assertResponseContains(t, "onboarding", onboarding, "Connect a provider. Let Kinosail handle the rest.", "Primary language", `aria-describedby="language-help"`, `value="en" selected`, `value="es-419"`, "needs permission to write", "Connect a subtitle provider", "Create a free account, open its API panel", "Create or sign in to an OpenSubtitles.com account", "open My Profile", `href="/settings/configuration#integrations.subdl.api_key"`, `href="/settings/configuration#integrations.opensubtitles"`, `href="/settings#provider"`, "Not configured", "Finish and open overview")
}

func assertResponseContains(t *testing.T, name string, response *httptest.ResponseRecorder, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("%s missing %q: %q", name, fragment, response.Body.String())
		}
	}
}

func TestSubtitleAppWritesValidatedSidecarBesideVideo(t *testing.T) { //nolint:cyclop // The provider request, sidecar write, conflict, and refreshed projection form one vertical slice.
	t.Parallel()
	var searches atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			searches.Add(1)
			if request.URL.Query().Get("api_key") != "key" || request.URL.Query().Get("film_name") != "Arrival" || request.URL.Query().Get("languages") != "EN" {
				http.Error(writer, "bad query", http.StatusBadRequest)
				return
			}
			writeSubDLSearch(writer, request.Host, "/arrival.srt", "Arrival")
		case "/arrival.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	video := filepath.Join(media, "Arrival.BluRay-GROUP.mp4")
	target := filepath.Join(media, "Arrival.BluRay-GROUP.en.srt")
	writeTestFile(t, video, "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	home := requestApp(t, handler, http.MethodGet, "/", "")
	match := regexp.MustCompile(`/subtitles/manage/([a-f0-9]+)/fetch`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatalf("missing fetch action: %q", home.Body.String())
	}

	fetched := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+match[1]+"/fetch", `{}`)
	data, err := os.ReadFile(target)
	if fetched.Code != http.StatusCreated || err != nil || !strings.Contains(string(data), "Hello") || searches.Load() != 1 {
		t.Fatalf("fetch = %d %q, sidecar = %q, err = %v, searches = %d", fetched.Code, fetched.Body.String(), data, err, searches.Load())
	}

	again := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+match[1]+"/fetch", `{}`)
	if again.Code != http.StatusConflict || searches.Load() != 1 {
		t.Fatalf("duplicate = %d %q, searches = %d", again.Code, again.Body.String(), searches.Load())
	}
	refreshed := requestApp(t, handler, http.MethodGet, "/?view=library", "")
	if !strings.Contains(refreshed.Body.String(), "Your subtitles are ready") && !strings.Contains(refreshed.Body.String(), ">Ready<") {
		t.Fatalf("refreshed dashboard = %q", refreshed.Body.String())
	}
}

func TestSubtitleSettingsKeepsSubtitleNavigation(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: t.TempDir(), CacheDir: t.TempDir()})
	response := requestApp(t, handler, http.MethodGet, "/settings", "")
	body := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("settings status = %d", response.Code)
	}
	for _, href := range []string{`href="/?view=summary"`, `href="/?view=wanted"`, `href="/?view=library"`} {
		if !strings.Contains(body, href) {
			t.Errorf("settings missing navigation %s", href)
		}
	}
	for _, href := range []string{`href="/?view=movies"`, `href="/?view=shows"`, `href="/?view=list"`} {
		if strings.Contains(body, href) {
			t.Errorf("settings leaks Player navigation %s", href)
		}
	}
}
