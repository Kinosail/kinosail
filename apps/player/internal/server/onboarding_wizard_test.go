package server_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
)

func wizardMedia(t *testing.T) (string, string) {
	t.Helper()
	media, outside := t.TempDir(), t.TempDir()
	for _, name := range []string{"Movies/Arrival (2016)/Arrival.mkv", "Movies/Dune (2021)/Dune.mp4", "TV Shows/Severance/Season 01/Severance.S01E01.mkv", ".hidden/Secret.mkv", "<em>Bold/Clip.mp4", "Movies/Arrival (2016)/poster.jpg"} {
		path := filepath.Join(media, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "Private.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(media, "Escape")); err != nil {
		t.Fatal(err)
	}
	return media, outside
}

func wizardOwner(t *testing.T, media string, environment ...string) (http.Handler, *http.Cookie, string) {
	t.Helper()
	data := t.TempDir()
	configured, err := configuration.Load(data, "", func(name string) (string, bool) {
		for index := 0; index+1 < len(environment); index += 2 {
			if environment[index] == name {
				return environment[index+1], true
			}
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true, Configuration: configured})
	setup := webFormCall(t, handler, "", "/setup", url.Values{"name": {"Owner"}, "password": {"owner-password"}})
	if setup.Code != http.StatusSeeOther || setup.Header().Get("Location") != "/account?setup=1" || len(setup.Result().Cookies()) != 1 {
		t.Fatalf("Owner setup = %d %q", setup.Code, setup.Header().Get("Location"))
	}
	return handler, setup.Result().Cookies()[0], data
}

func wizardForm(t *testing.T, handler http.Handler, cookie *http.Cookie, path string, form url.Values) *http.Response {
	t.Helper()
	return requestWithCookie(t, handler, http.MethodPost, path, form.Encode(), cookie).Result()
}

func TestOwnerPicksMediaFoldersDuringSetup(t *testing.T) {
	t.Parallel()
	media, _ := wizardMedia(t)
	handler, owner, _ := wizardOwner(t, media)

	prompt := requestWithCookie(t, handler, http.MethodGet, "/account?setup=1", "", owner)
	assertAPIBody(t, prompt, http.StatusOK, "Protect your account", "Skip for now", `href="/onboarding/media"`, `data-setup-next="/onboarding/media"`)
	library := requestWithCookie(t, handler, http.MethodGet, "/api/v1/library", "", owner)
	assertAPIBody(t, library, http.StatusOK, `"items"`)

	whole := requestWithCookie(t, handler, http.MethodGet, "/onboarding/media", "", owner)
	assertAPIBody(t, whole, http.StatusOK, "Your folders", "Already included")
	if strings.Contains(whole.Body.String(), `aria-label="Add Movies"`) {
		t.Fatalf("covered folder offered an Add button: %q", whole.Body.String())
	}
	if removed := wizardForm(t, handler, owner, "/onboarding/media/remove", url.Values{"path": {"."}}); removed.StatusCode != http.StatusSeeOther || removed.Header.Get("Location") != "/onboarding/media" {
		t.Fatalf("remove whole media folder = %d %q", removed.StatusCode, removed.Header.Get("Location"))
	}
	page := requestWithCookie(t, handler, http.MethodGet, "/onboarding/media", "", owner)
	assertAPIBody(t, page, http.StatusOK, "Where are your movies and shows?", "No folders yet", "Suggested folders", "Movies", "TV Shows", `name="path" value="Movies"`, "Skip for now", `aria-current="step"`, "&lt;em&gt;Bold")
	body := page.Body.String()
	for _, hidden := range []string{".hidden", "<em>Bold"} {
		if strings.Contains(body, hidden) {
			t.Fatalf("media picker exposed %q: %q", hidden, body)
		}
	}

	browse := requestWithCookie(t, handler, http.MethodGet, "/onboarding/media?path=Movies", "", owner)
	assertAPIBody(t, browse, http.StatusOK, "Arrival (2016)", "Dune (2021)", "Up one folder", "Add this folder", `name="path" value="Movies/Dune (2021)"`)

	if added := wizardForm(t, handler, owner, "/onboarding/media", url.Values{"path": {"Movies"}}); added.StatusCode != http.StatusSeeOther || added.Header.Get("Location") != "/onboarding/media" {
		t.Fatalf("add Movies = %d %q", added.StatusCode, added.Header.Get("Location"))
	}
	if added := wizardForm(t, handler, owner, "/onboarding/media", url.Values{"path": {"TV Shows"}}); added.StatusCode != http.StatusSeeOther {
		t.Fatalf("add TV Shows = %d", added.StatusCode)
	}
	assertAPIBody(t, requestWithCookie(t, handler, http.MethodGet, "/onboarding/media?path=Movies", "", owner), http.StatusOK, "Already in your folders")
	chosen := requestWithCookie(t, handler, http.MethodGet, "/onboarding/media", "", owner)
	assertAPIBody(t, chosen, http.StatusOK, "Your folders", "2 videos found", "1 video found", `action="/onboarding/media/remove"`, "Continue", `href="/onboarding/connection"`)
	assertAPIBody(t, requestWithCookie(t, handler, http.MethodGet, "/api/v1/settings", "", owner), http.StatusOK, `"Movies"`, `"TV Shows"`)
}

func TestMediaPickerRejectsUnsafeFoldersWithoutSideEffects(t *testing.T) {
	t.Parallel()
	media, outside := wizardMedia(t)
	handler, owner, _ := wizardOwner(t, media)
	wizardForm(t, handler, owner, "/onboarding/media/remove", url.Values{"path": {"."}})
	wizardForm(t, handler, owner, "/onboarding/media", url.Values{"path": {"Movies"}})
	before := requestWithCookie(t, handler, http.MethodGet, "/api/v1/settings", "", owner).Body.String()

	for name, folder := range map[string]string{
		"parent":    "../" + filepath.Base(outside),
		"absolute":  outside,
		"symlink":   "Escape",
		"missing":   "Nowhere",
		"empty":     "",
		"oversized": strings.Repeat("a", 4097),
		"overlap":   "Movies/Dune (2021)",
		"null":      "Movies\x00",
	} {
		response := wizardForm(t, handler, owner, "/onboarding/media", url.Values{"path": {folder}})
		if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/onboarding/media?message=unavailable" {
			t.Fatalf("%s add = %d %q", name, response.StatusCode, response.Header.Get("Location"))
		}
		browse := requestWithCookie(t, handler, http.MethodGet, "/onboarding/media?path="+url.QueryEscape(folder), "", owner)
		if browse.Code != http.StatusOK || strings.Contains(browse.Body.String(), "Private.mkv") || name != "empty" && name != "overlap" && !strings.Contains(browse.Body.String(), "That folder cannot be opened.") {
			t.Fatalf("%s browse = %d %q", name, browse.Code, browse.Body.String())
		}
	}
	notice := requestWithCookie(t, handler, http.MethodGet, "/onboarding/media?message=unavailable", "", owner)
	assertAPIBody(t, notice, http.StatusOK, "Kinosail could not use that folder.")
	if after := requestWithCookie(t, handler, http.MethodGet, "/api/v1/settings", "", owner).Body.String(); after != before {
		t.Fatalf("rejected folders changed settings:\nbefore %s\nafter  %s", before, after)
	}
}

func TestOwnerSharesKinosailWithHomeDevices(t *testing.T) {
	t.Parallel()
	media, _ := wizardMedia(t)
	handler, owner, _ := wizardOwner(t, media)

	devices := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	assertAPIBody(t, devices, http.StatusOK, "Watch on your TV and phone", `name="share" value="true" checked`, "Advanced options", "Step 3 of 5")
	saved := requestWithCookie(t, handler, http.MethodPost, "/onboarding/devices", "share=true", owner)
	assertAPIBody(t, saved, http.StatusOK, "Restart Kinosail to finish", `href="/onboarding/household"`)
	assertAPIBody(t, requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner), http.StatusOK, `value="0.0.0.0:38127"`, "Server listening address")
	again := wizardForm(t, handler, owner, "/onboarding/devices", url.Values{"share": {"true"}})
	if again.StatusCode != http.StatusSeeOther || again.Header.Get("Location") != "/onboarding/household" {
		t.Fatalf("unchanged sharing = %d %q", again.StatusCode, again.Header.Get("Location"))
	}
	private := requestWithCookie(t, handler, http.MethodPost, "/onboarding/devices", "", owner)
	assertAPIBody(t, private, http.StatusOK, "Restart Kinosail to finish")
	assertAPIBody(t, requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner), http.StatusOK, `value="127.0.0.1:38127"`)
}

func TestManagedListenAddressKeepsDeploymentChoice(t *testing.T) {
	t.Parallel()
	media, _ := wizardMedia(t)
	handler, owner, _ := wizardOwner(t, media, "KINOSAIL_LISTEN", "0.0.0.0:8096")

	devices := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	assertAPIBody(t, devices, http.StatusOK, "Kinosail is shared with devices on your home network.", "Your installation controls this setting.")
	if strings.Contains(devices.Body.String(), `name="share"`) {
		t.Fatalf("managed listen address offered a sharing switch: %q", devices.Body.String())
	}
	saved := wizardForm(t, handler, owner, "/onboarding/devices", url.Values{})
	if saved.StatusCode != http.StatusSeeOther || saved.Header.Get("Location") != "/onboarding/household" {
		t.Fatalf("managed sharing change = %d %q", saved.StatusCode, saved.Header.Get("Location"))
	}
	assertAPIBody(t, requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner), http.StatusOK, "<code>KINOSAIL_LISTEN</code>")
}

func TestUnprotectedOwnerMustAddSecondFactorBeforeSensitiveChanges(t *testing.T) {
	t.Parallel()
	media, _ := wizardMedia(t)
	handler, owner, _ := wizardOwner(t, media)

	settings := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	assertAPIBody(t, settings, http.StatusOK, "Protect your account", `href="/account"`)
	keys := requestWithCookie(t, handler, http.MethodPost, "/settings/api-keys", "name=Installer", owner)
	if keys.Code == http.StatusOK || keys.Code == http.StatusSeeOther && !strings.HasPrefix(keys.Header().Get("Location"), "/account") {
		t.Fatalf("unprotected Owner created an API key: %d %q", keys.Code, keys.Header().Get("Location"))
	}
	config := requestWithCookie(t, handler, http.MethodPost, "/settings/configuration", "key=listen&value=0.0.0.0%3A38127", owner)
	if config.Code == http.StatusSeeOther && !strings.HasPrefix(config.Header().Get("Location"), "/account") {
		t.Fatalf("unprotected Owner changed advanced configuration: %d %q", config.Code, config.Header().Get("Location"))
	}
	listing := requestWithCookie(t, handler, http.MethodGet, "/api/v1/api-keys", "", owner)
	assertAPIBody(t, listing, http.StatusOK, `[]`)
	assertAPIBody(t, requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner), http.StatusOK, `value="127.0.0.1:38127"`)
}
