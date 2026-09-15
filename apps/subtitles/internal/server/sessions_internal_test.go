package server

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestCleanDeviceNameNormalizesBrowserAgentsAndBoundsOtherNames(t *testing.T) {
	for input, want := range map[string]string{
		"  ":                            "Web browser",
		"Mozilla Firefox/128":           "Firefox",
		"Mozilla Edg/128 Chrome/128":    "Microsoft Edge",
		"Mozilla Chrome/128 Safari/537": "Chrome",
		"Mozilla Safari/537":            "Safari",
		"  Living Room TV  ":            "Living Room TV",
		strings.Repeat("x", 81):         strings.Repeat("x", 80),
	} {
		if got := cleanDeviceName(input); got != want {
			t.Fatalf("cleanDeviceName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOutboundNetworkRejectsLinkLocalMetadataAddresses(t *testing.T) {
	for raw, allowed := range map[string]bool{
		"169.254.169.254": false, "fe80::1": false, "127.0.0.1": false, "10.0.0.2": false,
		"100.64.0.1": false, "fc00::1": false, "::ffff:10.0.0.2": false, "203.0.113.10": true,
	} {
		if got := allowedOutboundIP(net.ParseIP(raw)); got != allowed {
			t.Fatalf("allowedOutboundIP(%q) = %v", raw, got)
		}
	}
}

func TestPublicOutboundResolutionRejectsMixedAndEmptyAnswers(t *testing.T) {
	checkResolution := servertest.PublicOutboundResolutionRejectsMixedAndEmptyAnswers
	checkResolution(t, allowedOutboundIP)
}

func TestBrowserSessionsHaveIdleAndAbsoluteTimeouts(t *testing.T) {
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil || store.addOwner(profile) != nil {
		t.Fatal(err)
	}
	token, err := store.createSessionKind(profile.ID, "Browser", true, false)
	if err != nil {
		t.Fatal(err)
	}
	key := sessionKey(token)
	session := store.sessions[key]
	if !session.Browser || time.Unix(session.ExpiresAt, 0).Sub(time.Unix(session.CreatedAt, 0)) > defaultSessionAbsolute+time.Second {
		t.Fatalf("browser session = %+v", session)
	}
	session.LastSeen = time.Now().Add(-defaultSessionInactive - time.Minute).Unix()
	store.sessions[key] = session
	request := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
	request.AddCookie(sessionCookie(token))
	if _, ok := store.profile(request); ok {
		t.Fatal("idle browser session was accepted")
	}
}

func TestBrowserSessionsUseCustomizedIdleAndAbsoluteTimeouts(t *testing.T) {
	settings := newSettingsStore("", "", "", nil)
	if err := settings.setSessionTimeouts(24, 168); err != nil {
		t.Fatal(err)
	}
	store := newProfileStore(t.TempDir())
	store.sessionTimeouts = settings.sessionTimeouts
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil || store.addOwner(profile) != nil {
		t.Fatal(err)
	}
	token, err := store.createSessionKind(profile.ID, "Browser", true, false)
	if err != nil {
		t.Fatal(err)
	}
	key := sessionKey(token)
	session := store.sessions[key]
	if lifetime := time.Unix(session.ExpiresAt, 0).Sub(time.Unix(session.CreatedAt, 0)); lifetime > 7*24*time.Hour+time.Second {
		t.Fatalf("session lifetime = %s", lifetime)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(sessionCookie(token))
	session.LastSeen = time.Now().Add(-25 * time.Hour).Unix()
	store.sessions[key] = session
	if _, ok := store.profile(request); ok {
		t.Fatal("custom inactivity timeout was ignored")
	}
	if store.activeSessions() != 0 || len(store.devices()) != 0 {
		t.Fatal("expired browser session remained visible as an active device")
	}
}

func TestSessionTimeoutValidationHasNoSideEffects(t *testing.T) {
	store := newSettingsStore("", "", "", nil)
	checkTimeouts := servertest.SessionTimeoutValidationHasNoSideEffects
	checkTimeouts(t, store.sessionTimeouts, store.setSessionTimeouts)
}

func TestSensitiveActionsRequireRecentHumanAuthentication(t *testing.T) {
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil || store.addOwner(profile) != nil {
		t.Fatal(err)
	}
	token, err := store.createStrongSession(profile.ID, "Browser", true)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), "POST", "/settings/server", nil)
	request.AddCookie(sessionCookie(token))
	if !store.recentlyAuthenticated(request, 10*time.Minute) {
		t.Fatal("fresh human session was not strong")
	}
	session := store.sessions[sessionKey(token)]
	session.StrongAt = time.Now().Add(-11 * time.Minute).Unix()
	store.sessions[sessionKey(token)] = session
	if store.recentlyAuthenticated(request, 10*time.Minute) {
		t.Fatal("stale human session remained strong")
	}
}

func TestPublicSessionsAreStrongShortLivedAndBounded(t *testing.T) { //nolint:cyclop // Every public-session invariant is asserted in one fixture.
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Viewer", "viewer-password", false)
	if err != nil || store.addOwner(profile) != nil {
		t.Fatal(err)
	}
	store.profiles[0].Remote, store.profiles[0].TOTPSecret = true, "secret"
	for range 10 {
		token, createErr := store.createStrongPublicSession(profile.ID, "Browser", true)
		if createErr != nil {
			t.Fatal(createErr)
		}
		session := store.sessions[sessionKey(token)]
		if session.Channel != "public" || session.StrongAt == 0 || time.Unix(session.ExpiresAt, 0).Sub(time.Unix(session.CreatedAt, 0)) > 8*time.Hour+time.Second {
			t.Fatalf("public session = %+v", session)
		}
	}
	if _, err := store.createStrongPublicSession(profile.ID, "Browser", true); err == nil {
		t.Fatal("eleventh active public session was accepted")
	}
	cookie := publicSessionCookie("token")
	if cookie.Name != "__Host-kinosail_session" || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" || cookie.MaxAge != 8*60*60 || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("public cookie = %+v", cookie)
	}
}

func TestBrowserSessionCookiesAreAlwaysHostBoundAndSecure(t *testing.T) {
	cookie := sessionCookie("token")
	if cookie.Name != "__Host-kinosail_session" || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("browser cookie = %+v", cookie)
	}
}

func TestPublicSessionCannotSurviveProfilePolicyRevision(t *testing.T) {
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Viewer", "viewer-password", false)
	if err != nil || store.addOwner(profile) != nil {
		t.Fatal(err)
	}
	store.profiles[0].Owner = false
	store.profiles[0].Remote, store.profiles[0].TOTPSecret = true, "secret"
	token, err := store.createStrongPublicSession(profile.ID, "Browser", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.setProfile(profile.ID, false, profilePolicy{Remote: true, Rating: "family", Libraries: []string{"all"}}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(publicSessionCookie(token))
	if _, ok := store.profile(request); ok {
		t.Fatal("public session survived a profile-policy revision")
	}
}

func TestSessionCreationRejectsInactiveAndInvalidPublicProfiles(t *testing.T) {
	for name, profile := range map[string]viewerProfile{
		"disabled":    {ID: "viewer", Disabled: true},
		"deleted":     {ID: "viewer", SCIMDeleted: true},
		"owner":       {ID: "viewer", Owner: true, Remote: true, TOTPSecret: "secret"},
		"local only":  {ID: "viewer", TOTPSecret: "secret"},
		"not secured": {ID: "viewer", Remote: true},
	} {
		t.Run(name, func(t *testing.T) {
			store := &profileStore{profiles: []viewerProfile{profile}, sessions: map[string]viewerSession{}, persist: func(string, any) error { return nil }}
			if _, err := store.createStrongPublicSession(profile.ID, "Browser", true); err == nil {
				t.Fatal("invalid public profile received a session")
			}
			if len(store.sessions) != 0 {
				t.Fatalf("rejected profile changed sessions: %+v", store.sessions)
			}
		})
	}
	store := &profileStore{profiles: []viewerProfile{{ID: "disabled", Disabled: true}}, sessions: map[string]viewerSession{}, persist: func(string, any) error { return nil }}
	if _, err := store.createSession("disabled", "Device"); err == nil {
		t.Fatal("disabled profile received a local session")
	}
}

func TestProfilePolicyChangeDoesNotPersistBeforeSessionRevocation(t *testing.T) {
	profile := viewerProfile{ID: "viewer", Name: "Viewer", Remote: true, Revision: 1}
	session := viewerSession{ProfileID: profile.ID, Channel: "public", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	store := &profileStore{
		file:        "profiles.json",
		sessionFile: "sessions.json",
		apiFile:     "api_keys.json",
		profiles:    []viewerProfile{profile},
		sessions:    map[string]viewerSession{"session": session},
		apiKeys:     map[string]apiKey{},
		persist: func(path string, _ any) error {
			if path == "sessions.json" {
				return errors.New("forced session persistence failure")
			}
			return nil
		},
	}

	if err := store.setProfile(profile.ID, false, profilePolicy{Rating: "all", Libraries: []string{"all"}}); err == nil {
		t.Fatal("profile policy change succeeded without durable session revocation")
	}
	if !store.profiles[0].Remote || store.profiles[0].Revision != profile.Revision {
		t.Fatalf("failed policy change modified profile: %+v", store.profiles[0])
	}
	if _, found := store.sessions["session"]; !found {
		t.Fatal("failed policy change modified in-memory sessions")
	}
}

func TestFactorChangesRevokePublicSessions(t *testing.T) {
	for name, change := range map[string]func(*profileStore, string) error{
		"enable":  func(store *profileStore, id string) error { return store.enableMFA(id, "secret", []string{"recovery"}) },
		"disable": func(store *profileStore, id string) error { return store.disableMFA(id) },
	} {
		t.Run(name, func(t *testing.T) {
			profile := viewerProfile{ID: "viewer", TOTPSecret: map[bool]string{true: "secret"}[name == "disable"], Revision: 1}
			store := &profileStore{
				file: "profiles.json", sessionFile: "sessions.json", apiFile: "api_keys.json",
				profiles: []viewerProfile{profile}, sessions: map[string]viewerSession{
					"public": {ProfileID: profile.ID, Channel: "public"},
					"local":  {ProfileID: profile.ID},
				}, apiKeys: map[string]apiKey{}, persist: func(string, any) error { return nil },
			}

			if err := change(store, profile.ID); err != nil {
				t.Fatal(err)
			}
			if _, found := store.sessions["public"]; found {
				t.Fatal("public session survived factor change")
			}
			if _, found := store.sessions["local"]; !found || store.profiles[0].Revision != profile.Revision+1 {
				t.Fatalf("factor change state = profile %+v, sessions %+v", store.profiles[0], store.sessions)
			}
		})
	}
}
