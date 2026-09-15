package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAPISetupSettingsFailureDoesNotClaimFirstOwner(t *testing.T) {
	profiles := &profileStore{
		file:        "profiles.json",
		sessionFile: "sessions.json",
		apiFile:     "api_keys.json",
		sessions:    make(map[string]viewerSession),
		apiKeys:     make(map[string]apiKey),
		persist:     func(string, any) error { return nil },
		sessionTimeouts: func() (time.Duration, time.Duration) {
			return defaultSessionInactive, defaultSessionAbsolute
		},
	}
	settingsFailure := true
	settings := &settingsStore{
		file: "settings.json",
		value: installationSettings{
			Name:         "Kinosail",
			Libraries:    []string{"."},
			RequireMFA:   true,
			UpdateChecks: true,
		},
		persist: func(string, any) error {
			if settingsFailure {
				settingsFailure = false
				return errors.New("forced settings failure")
			}
			return nil
		},
	}
	auth := &authentication{profiles: profiles, settings: settings, mfa: newMFA(profiles)}

	first := setupAPIRequest(t, auth)
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("failed setup = %d %q", first.Code, first.Body.String())
	}
	if profiles.hasProfiles() {
		t.Fatal("failed setup claimed the first Owner")
	}
	second := setupAPIRequest(t, auth)
	if second.Code != http.StatusCreated {
		t.Fatalf("setup retry = %d %q", second.Code, second.Body.String())
	}
}

func TestAPISetupOwnerFailureRollsBackSettingsAndMFAEnrollment(t *testing.T) {
	profiles := &profileStore{
		file:        "profiles.json",
		sessionFile: "sessions.json",
		apiFile:     "api_keys.json",
		sessions:    make(map[string]viewerSession),
		apiKeys:     make(map[string]apiKey),
		persist:     func(string, any) error { return errors.New("forced Owner failure") },
	}
	initial := installationSettings{Name: "Kinosail", Libraries: []string{"."}, RequireMFA: true, UpdateChecks: true}
	persisted := initial
	settings := &settingsStore{
		file:  "settings.json",
		value: initial,
		persist: func(_ string, value any) error {
			persisted = value.(installationSettings)
			return nil
		},
	}
	auth := &authentication{profiles: profiles, settings: settings, mfa: newMFA(profiles)}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", strings.NewReader(`{"name":"Owner","password":"owner-password","totp":true}`))
	response := httptest.NewRecorder()
	auth.createAPISetup(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed setup = %d %q", response.Code, response.Body.String())
	}
	if profiles.hasProfiles() {
		t.Fatal("failed setup claimed the first Owner")
	}
	if settings.snapshot().OnboardingPending || persisted.OnboardingPending {
		t.Fatal("failed setup retained onboarding state")
	}
	if auth.mfa.engine.PendingCount() != 0 {
		t.Fatal("failed setup retained an MFA enrollment")
	}
}

func TestAPIRequireMFASettingsFailureKeepsActiveSessions(t *testing.T) {
	now := time.Now()
	profiles := &profileStore{
		profiles:    []viewerProfile{{ID: "owner", Name: "Owner", Owner: true}},
		sessionFile: "sessions.json",
		sessions: map[string]viewerSession{
			sessionKey("current-session"): {ProfileID: "owner", ExpiresAt: now.Add(time.Hour).Unix()},
			sessionKey("other-session"):   {ProfileID: "owner", ExpiresAt: now.Add(time.Hour).Unix()},
		},
		apiKeys: make(map[string]apiKey),
		persist: func(string, any) error { return nil },
		sessionTimeouts: func() (time.Duration, time.Duration) {
			return defaultSessionInactive, defaultSessionAbsolute
		},
	}
	settings := &settingsStore{
		file:    "settings.json",
		persist: func(string, any) error { return errors.New("forced settings failure") },
	}
	auth := &authentication{profiles: profiles, settings: settings}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/mfa", strings.NewReader(`{"required":true}`))
	response := httptest.NewRecorder()
	apiMFARequirement(auth).ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed MFA setting = %d %q", response.Code, response.Body.String())
	}
	if settings.requireMFA() {
		t.Fatal("failed MFA setting changed the requirement")
	}
	for _, token := range []string{"current-session", "other-session"} {
		probe := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
		probe.Header.Set("Authorization", "Bearer "+token)
		if _, found := profiles.profile(probe); !found {
			t.Fatalf("failed MFA setting revoked %s", token)
		}
	}
}

func TestAPIRequireMFASessionFailureRollsBackRequirement(t *testing.T) {
	now := time.Now()
	profiles := &profileStore{
		sessionFile: "sessions.json",
		sessions: map[string]viewerSession{
			sessionKey("current-session"): {ProfileID: "owner", ExpiresAt: now.Add(time.Hour).Unix()},
		},
		persist: func(string, any) error { return errors.New("forced session failure") },
	}
	persisted := installationSettings{}
	settings := &settingsStore{
		file: "settings.json",
		persist: func(_ string, value any) error {
			persisted = value.(installationSettings)
			return nil
		},
	}
	auth := &authentication{profiles: profiles, settings: settings}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/mfa", strings.NewReader(`{"required":true}`))
	response := httptest.NewRecorder()
	apiMFARequirement(auth).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed MFA setting = %d %q", response.Code, response.Body.String())
	}
	if settings.requireMFA() || persisted.RequireMFA {
		t.Fatal("failed session revocation retained the MFA requirement")
	}
	if len(profiles.sessions) != 1 {
		t.Fatal("failed session revocation changed active sessions")
	}
}

func TestAuthStateMutationsUseOneLockOrder(t *testing.T) {
	for iteration := 0; iteration < 50; iteration++ {
		profiles := &profileStore{
			file:        "profiles.json",
			sessionFile: "sessions.json",
			sessions:    make(map[string]viewerSession),
			persist:     func(string, any) error { return nil },
		}
		settings := &settingsStore{file: "settings.json", persist: func(string, any) error { return nil }}
		auth := &authentication{profiles: profiles, settings: settings}
		completed := make(chan error, 2)
		go func() { completed <- auth.commitFirstOwner(t.Context(), viewerProfile{ID: "owner", Owner: true}, true) }()
		go func() {
			_, err := auth.setRequireMFA(t.Context(), true)
			completed <- err
		}()
		for range 2 {
			select {
			case err := <-completed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("concurrent authentication state changes deadlocked")
			}
		}
		if !settings.requireMFA() || len(profiles.list()) != 1 {
			t.Fatal("concurrent authentication state changes lost state")
		}
	}
}

func setupAPIRequest(t *testing.T, auth *authentication) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", strings.NewReader(`{"name":"Owner","password":"owner-password"}`))
	response := httptest.NewRecorder()
	auth.createAPISetup(response, request)
	return response
}
