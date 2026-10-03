package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPasswordLoginCreatesRecentAuthentication(t *testing.T) {
	dataDir := t.TempDir()
	profiles := newProfileStore(dataDir)
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = addTestOwner(profiles, profile); err != nil {
		t.Fatal(err)
	}
	auth := &authentication{profiles: profiles, settings: newSettingsStore("", dataDir, "", nil)}
	form := url.Values{"name": {"Owner"}, "password": {"owner-password"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login?stepup=1", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	auth.login(response, request)
	check := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings", nil)
	check.AddCookie(response.Result().Cookies()[0])
	if !profiles.recentlyAuthenticated(check, 10*time.Minute) {
		t.Fatal("fresh password login did not satisfy recent authentication")
	}
}

func TestOwnerSetupCreatesRecentAuthentication(t *testing.T) {
	dataDir := t.TempDir()
	profiles := newProfileStore(dataDir)
	auth := &authentication{profiles: profiles, settings: newSettingsStore("", dataDir, "", nil), mfa: newMFA(profiles)}
	form := url.Values{"name": {"Owner"}, "password": {"owner-password"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	auth.setup(response, request)
	check := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/onboarding/trusted-https", nil)
	check.AddCookie(response.Result().Cookies()[0])
	if !profiles.recentlyAuthenticated(check, 10*time.Minute) {
		t.Fatal("fresh Owner setup did not satisfy recent authentication")
	}
}
