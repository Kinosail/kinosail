package server_test

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestOwnerDownloadsVerifiedDirectWireGuardProfile(t *testing.T) { //nolint:cyclop,funlen,gocognit // One interface scenario proves create, disclose, and revoke through web and API adapters.
	t.Parallel()

	handler := server.New(server.Config{
		MediaDir: t.TempDir(), DataDir: t.TempDir(), CacheDir: t.TempDir(),
		WireGuardDir: t.TempDir(), WireGuardEndpoint: "media.example.com:51820", RequireAuth: true,
	})
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "Test", "totp": true})
	var session struct {
		Token string
		TOTP  struct{ Secret string }
	}
	mustJSON(t, setup, &session)
	if response := apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, session.TOTP.Secret, time.Now())}); response.Code != http.StatusOK {
		t.Fatalf("MFA = %d %q", response.Code, response.Body.String())
	}
	profilesResponse := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/profiles", nil)
	var profiles struct {
		Profiles []struct {
			ID    string
			Owner bool
		}
	}
	mustJSON(t, profilesResponse, &profiles)
	for name, profileID := range map[string]string{"missing": "", "Owner": profiles.Profiles[0].ID} {
		if rejected := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/remote-access/wireguard", map[string]string{"label": "Rejected " + name, "profileId": profileID}); rejected.Code != http.StatusBadRequest {
			t.Fatalf("%s pairing = %d %q", name, rejected.Code, rejected.Body.String())
		}
	}
	if remote := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/remote-access", nil); strings.Contains(remote.Body.String(), `"publicKey"`) {
		t.Fatalf("rejected binding created a peer: %q", remote.Body.String())
	}
	createdProfile := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password", "rating": "all", "libraries": []string{"all"}})
	var profile struct{ ID string }
	mustJSON(t, createdProfile, &profile)
	if createdProfile.Code != http.StatusCreated || profile.ID == "" {
		t.Fatalf("Viewer Profile = %d %q", createdProfile.Code, createdProfile.Body.String())
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/remote/wireguard", strings.NewReader("label=Family+iPhone&profileId="+url.QueryEscape(profile.ID)))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer "+session.Token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Content-Type") != "application/x-wireguard-profile" ||
		!strings.Contains(response.Body.String(), "Endpoint = media.example.com:51820") || !strings.Contains(response.Body.String(), "AllowedIPs = 10.91.0.1/32") {
		t.Fatalf("pairing = %d %q %#v", response.Code, response.Body.String(), response.Header())
	}
	settings := httptest.NewRecorder()
	settingsRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	settingsRequest.Header.Set("Authorization", "Bearer "+session.Token)
	handler.ServeHTTP(settings, settingsRequest)
	if !strings.Contains(settings.Body.String(), "Verified Direct Connection") {
		t.Fatalf("settings = %q", settings.Body.String())
	}
	match := regexp.MustCompile(`action="/settings/remote/wireguard/revoke"[\s\S]*?value="([^"]+)"`).FindStringSubmatch(settings.Body.String())
	if len(match) != 2 || !strings.Contains(settings.Body.String(), "Family iPhone") {
		t.Fatalf("WireGuard Viewer is not manageable: %q", settings.Body.String())
	}
	revoke := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/remote/wireguard/revoke", strings.NewReader("publicKey="+url.QueryEscape(html.UnescapeString(match[1]))))
	revoke.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	revoke.Header.Set("Authorization", "Bearer "+session.Token)
	revoked := httptest.NewRecorder()
	handler.ServeHTTP(revoked, revoke)
	if revoked.Code != http.StatusSeeOther {
		t.Fatalf("revoke = %d %q", revoked.Code, revoked.Body.String())
	}
	created := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/remote-access/wireguard", map[string]string{"label": "Family iPad", "profileId": profile.ID})
	remote := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/remote-access", nil)
	match = regexp.MustCompile(`"publicKey":"([^"]+)"`).FindStringSubmatch(remote.Body.String())
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), "Endpoint = media.example.com:51820") || len(match) != 2 || !strings.Contains(remote.Body.String(), `"enabled":true`) || !strings.Contains(remote.Body.String(), `"profileId":"`+profile.ID+`"`) {
		t.Fatalf("WireGuard API create = %d %q, remote = %d %q", created.Code, created.Body.String(), remote.Code, remote.Body.String())
	}
	deleted := apiCall(t, handler, session.Token, http.MethodDelete, "/api/v1/remote-access/wireguard", map[string]string{"publicKey": match[1]})
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("WireGuard API revoke = %d %q", deleted.Code, deleted.Body.String())
	}
}
