package server

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

const (
	defaultSessionInactive = identitycore.DefaultSessionInactive
	defaultSessionAbsolute = identitycore.DefaultSessionAbsolute
)

type (
	viewerSession = identitycore.Session
	deviceSession = identitycore.Device
)

func loadSessions(path string, databases ...*database.Store) (map[string]viewerSession, error) {
	return identitycore.LoadSessions(path, configuredDatabase(databases), nil)
}

func (store *profileStore) sessionModule() *identitycore.RequestSessions {
	return identitycore.NewRequestSessions(identitycore.SessionConfig{
		Mutex: &store.mu, Values: &store.sessions, File: store.sessionFile, Persist: store.persist,
		Profiles: store.sessionProfiles, Timeouts: store.sessionTimeouts,
	}, sessionToken)
}

func (store *profileStore) sessionProfiles() []identitycore.SessionProfile {
	profiles := make([]identitycore.SessionProfile, 0, len(store.profiles))
	for _, profile := range store.profiles {
		profiles = append(profiles, identitycore.SessionProfile{
			ID: profile.ID, Name: profile.Name, Owner: profile.Owner, Remote: profile.Remote,
			Secured: profile.Secured(), Disabled: profile.Disabled, Deleted: profile.SCIMDeleted, Revision: profile.Revision,
		})
	}
	return profiles
}

func (store *profileStore) createSession(id, name string) (string, error) {
	return store.createSessionKind(id, name, false, false)
}

func (store *profileStore) createCompatibilitySession(id, name string) (string, error) {
	return store.sessionModule().CreateCompatibility(id, name)
}

func compatibilityProfile(profile viewerProfile) viewerProfile {
	if profile.Owner {
		profile.Owner, profile.Downloads, profile.Transcode, profile.Remote = false, true, true, false
		profile.Rating, profile.Libraries = "all", []string{"all"}
	}
	return profile
}

func (store *profileStore) createStrongSession(id, name string, browser bool) (string, error) {
	return store.sessionModule().CreateStrong(id, name, browser)
}

func (store *profileStore) createSessionKind(id, name string, browser, strong bool) (string, error) {
	return store.sessionModule().CreateLocal(id, name, browser, strong)
}

func (store *profileStore) signIn(writer http.ResponseWriter, request *http.Request, id string) error {
	return store.sessionModule().SignIn(writer, request, id)
}

func (store *profileStore) signInStrong(writer http.ResponseWriter, request *http.Request, id string) error {
	return store.sessionModule().SignInStrong(writer, request, id)
}

func (store *profileStore) signInStrongPublic(writer http.ResponseWriter, request *http.Request, id string) error {
	return store.sessionModule().SignInStrongPublic(writer, request, id)
}

func (store *profileStore) markStrong(request *http.Request) error {
	return store.sessionModule().MarkStrong(request)
}

func (store *profileStore) recentlyAuthenticated(request *http.Request, maximumAge time.Duration) bool {
	return store.sessionModule().RecentlyAuthenticated(request, maximumAge)
}

func (store *profileStore) publicSession(request *http.Request) bool {
	return store.sessionModule().Public(request)
}

func (store *profileStore) revokePublicSessions() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	profiles := cloneProfiles(store.profiles)
	for index := range profiles {
		if !profiles[index].Owner && profiles[index].Remote {
			profiles[index].Revision++
		}
	}
	sessions := identitycore.WithoutPublicSessions(store.sessions)
	if err := store.saveRelated(profiles, sessions, store.apiKeys); err != nil {
		return err
	}
	store.profiles, store.sessions = profiles, sessions
	return nil
}

func sessionCookie(token string) *http.Cookie { return identitycore.SessionCookie(token) }

func publicSessionCookie(token string) *http.Cookie {
	return identitycore.PublicSessionCookie(token, time.Now())
}

func (store *profileStore) signOut(request *http.Request) error {
	return store.sessionModule().SignOut(request)
}

func withoutProfile(sessions map[string]viewerSession, profileID string) map[string]viewerSession {
	return identitycore.WithoutProfile(sessions, profileID, false)
}

func withoutPublicProfileSessions(sessions map[string]viewerSession, profileID string) map[string]viewerSession {
	return identitycore.WithoutProfile(sessions, profileID, true)
}
