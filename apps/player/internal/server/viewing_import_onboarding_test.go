package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

var onboardingFixture = servertest.OnboardingFixture{Library: libraryAPIFixture, Web: requestWithCookie, TOTP: testTOTP, Introduction: "Set up your Server", PlanLabel: "Household"}

func TestOwnerSetupContinuesToConnectionOnboarding(t *testing.T) {
	onboardingFixture.OwnerSetupContinuesToConnectionOnboarding(t)
}

func TestOwnerAuthenticatorChoiceRetainsConnectionOnboarding(t *testing.T) {
	onboardingFixture.OwnerAuthenticatorChoiceRetainsConnectionOnboarding(t)
}

func TestOwnerCanRerunOnboardingFromSettings(t *testing.T) {
	onboardingFixture.OwnerCanRerunOnboardingFromSettings(t)
}

func TestOwnerFirstCredentialLoginOpensOnboarding(t *testing.T) {
	onboardingFixture.OwnerFirstCredentialLoginOpensOnboarding(t)
}

func TestOwnerCanStartViewingMigrationFromOnboarding(t *testing.T) { //nolint:cyclop,funlen // One onboarding scenario covers preview-safe migration entry.
	const sourceToken = "onboarding-source-secret"
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != sourceToken {
			http.Error(writer, "missing token", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/Users/Me":
			_, _ = writer.Write([]byte(`{"Id":"viewer"}`))
		case "/Items":
			_, _ = writer.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"arrival","Name":"Arrival","Type":"Movie","ProductionYear":2016,"UserData":{"Played":true,"IsFavorite":true,"LastPlayedDate":"2026-08-22T12:00:00Z"}}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer upstream.Close()

	handler, token := apiServer(t)
	connection := apiCall(t, handler, token, http.MethodGet, "/onboarding/connection", nil)
	assertAPIBody(t, connection, http.StatusOK, "Choose how devices connect.", "Secure local access", "On by default.", "Jellyfin apps", "Trusted HTTPS is required.", "Trusted HTTPS (Required for Jellyfin apps)", "Jellyfin routes stay unavailable.", "Remote access comes later", `id="trusted-https-configuration"`, `href="#trusted-https-configuration"`, `action="/onboarding/jellyfin"`, `action="/onboarding/trusted-https"`, `href="/onboarding/household"`)
	household := apiCall(t, handler, token, http.MethodGet, "/onboarding/household", nil)
	assertAPIBody(t, household, http.StatusOK, "Set up Viewer Profiles.", "Parent or guardian", "Child or teen", `action="/onboarding/household"`, `name="kind" value="parent"`, `name="kind" value="child"`)
	parent := webFormCall(t, handler, token, "/onboarding/household", url.Values{"kind": {"parent"}, "owner": {"false"}, "rating": {"all"}, "libraries": {"all"}, "name": {"Parent"}, "password": {"parent-password"}})
	if parent.Code != http.StatusSeeOther || parent.Header().Get("Location") != "/onboarding/household" {
		t.Fatalf("add parent = %d, location = %q", parent.Code, parent.Header().Get("Location"))
	}
	household = apiCall(t, handler, token, http.MethodGet, "/onboarding/household", nil)
	assertAPIBody(t, household, http.StatusOK, "Parent", "Viewer")
	child := webFormCall(t, handler, token, "/onboarding/household", url.Values{"kind": {"child"}, "owner": {"false"}, "rating": {"teen"}, "libraries": {"all"}, "name": {"Child"}, "password": {"child-password"}})
	if child.Code != http.StatusSeeOther || child.Header().Get("Location") != "/onboarding/household" {
		t.Fatalf("add child = %d, location = %q", child.Code, child.Header().Get("Location"))
	}
	household = apiCall(t, handler, token, http.MethodGet, "/onboarding/household", nil)
	assertAPIBody(t, household, http.StatusOK, "Parent", "Child", "Viewer")
	profilesBeforeReject := apiCall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil).Body.String()
	rejected := webFormCall(t, handler, token, "/onboarding/household", url.Values{"kind": {"parent"}, "owner": {"true"}, "rating": {"all"}, "libraries": {"all"}, "name": {"Should Not Exist"}, "password": {"rejected-password"}})
	if rejected.Code != http.StatusBadRequest || strings.Contains(apiCall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil).Body.String(), "Should Not Exist") || profilesBeforeReject == "" {
		t.Fatalf("invalid parent = %d, profiles before=%q", rejected.Code, profilesBeforeReject)
	}
	page := apiCall(t, handler, token, http.MethodGet, "/onboarding/migrate", nil)
	assertAPIBody(t, page, http.StatusOK, "Import your viewing history.", "Plex", "Jellyfin", "watched state", "resume positions", "favorites", "become My List entries", "video playlists", "one Viewer Profile at a time", "not Universal Watchlist entries", `name="overwriteExisting"`, `action="/onboarding/viewing-imports/preview"`, `href="/onboarding/finish"`)
	if strings.Contains(page.Body.String(), "source token") {
		t.Fatalf("onboarding page contains a source credential: %q", page.Body.String())
	}
	profiles := apiCall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil)
	profileID := regexp.MustCompile(`"id":"([^"]+)"`).FindStringSubmatch(profiles.Body.String())[1]
	form := url.Values{"source": {"jellyfin"}, "url": {upstream.URL}, "token": {sourceToken}, "profileId": {profileID}}
	previewRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/onboarding/viewing-imports/preview", strings.NewReader(form.Encode()))
	previewRequest.Header.Set("Authorization", "Bearer "+token)
	previewRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	preview := httptest.NewRecorder()
	handler.ServeHTTP(preview, previewRequest)
	assertAPIBody(t, preview, http.StatusOK, "Step 4 of 4", `action="/onboarding/viewing-imports/apply"`, "Arrival", "favorite")
	if strings.Contains(preview.Body.String(), "Import and sync automatically") {
		t.Fatalf("onboarding preview offered recurring credential retention: %q", preview.Body.String())
	}
	if strings.Contains(preview.Body.String(), sourceToken) {
		t.Fatalf("onboarding preview exposed the source token: %q", preview.Body.String())
	}
	previewID := regexp.MustCompile(`name="id" value="([^"]+)"`).FindStringSubmatch(preview.Body.String())[1]
	applyForm := url.Values{"id": {previewID}}
	applyRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/onboarding/viewing-imports/apply", strings.NewReader(applyForm.Encode()))
	applyRequest.Header.Set("Authorization", "Bearer "+token)
	applyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, applyRequest)
	assertAPIBody(t, result, http.StatusOK, "Migration complete", "Viewing activity updated: 1", "Favorites or playlist entries added: 1", "Open Library")
}
