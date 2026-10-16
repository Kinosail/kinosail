package server

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

type tokenAccess struct {
	profile                viewerProfile
	session                viewerSession
	resolved, sessionFound bool
}

type tokenOutcome int

const (
	tokenDenied tokenOutcome = iota
	tokenAccepted
	tokenSettle
)

func (store *profileStore) access(request *http.Request) (tokenAccess, bool) {
	token := sessionToken(request)
	if token == "" {
		return tokenAccess{}, false
	}
	key, now := sessionKey(token), time.Now().Unix()
	store.mu.RLock()
	access, outcome := store.evaluateToken(request, key, now, false)
	store.mu.RUnlock()
	if outcome == tokenSettle {
		store.mu.Lock()
		defer store.mu.Unlock()
		access, outcome = store.evaluateToken(request, key, now, true)
	}
	return access, outcome == tokenAccepted
}

func (store *profileStore) evaluateToken(request *http.Request, key string, now int64, settle bool) (tokenAccess, tokenOutcome) {
	ctx := request.Context()
	session, found := store.sessions[key]
	if found && !identitycore.SessionMatchesRequest(session, request) || !found && identitycore.ManagementDeviceKey(request) != "" {
		return tokenAccess{}, tokenDenied
	}
	session = store.accessUse.Session(key, session, now)
	api, apiFound := store.apiKeys[key]
	if apiFound && api.ExpiresAt != 0 && api.ExpiresAt <= now {
		if !settle {
			return tokenAccess{}, tokenSettle
		}
		delete(store.apiKeys, key)
		store.saveAccessState(ctx, "api-key-expire", store.apiFile, now)
		apiFound = false
	}
	if !apiFound && (!found || store.sessionExpired(session, now)) {
		if found && !settle {
			return tokenAccess{}, tokenSettle
		}
		if found {
			delete(store.sessions, key)
			store.saveAccessState(ctx, "session-expire", store.sessionFile, now)
		}
		return tokenAccess{}, tokenDenied
	}
	api = store.accessUse.APIKey(key, api, now)
	profileID := session.ProfileID
	if apiFound {
		profileID = api.ProfileID
	}
	return store.profileForToken(ctx, now, profileID, key, session, found, api, apiFound, settle)
}

func (store *profileStore) profileForToken(ctx context.Context, now int64, profileID, key string, session viewerSession, sessionFound bool, api apiKey, apiFound, settle bool) (tokenAccess, tokenOutcome) {
	profile, exists := identitycore.FindProfile(store.profiles, profileID)
	if !exists || profile.Disabled || profile.SCIMDeleted {
		return tokenAccess{}, tokenDenied
	}
	if sessionFound && (session.Channel == "public" || session.ManagementDevice != "") && session.ProfileRevision != profile.Revision {
		if !settle {
			return tokenAccess{}, tokenSettle
		}
		delete(store.sessions, key)
		store.saveAccessState(ctx, "session-revoke", store.sessionFile, now)
		return tokenAccess{}, tokenDenied
	}
	store.accessUse.Record(key, session, sessionFound, api, apiFound, now)
	profile.APIKey, profile.Scopes = apiFound, append([]string(nil), api.Scopes...)
	if sessionFound && session.Channel == "compatibility" && profile.Owner {
		profile = compatibilityProfile(profile)
	}
	return tokenAccess{profile: profile, session: session, resolved: true, sessionFound: sessionFound}, tokenAccepted
}

func (store *profileStore) sessionExpired(session viewerSession, now int64) bool {
	inactive, absolute := identitycore.SessionTimeouts(session, store.sessionTimeouts, store.publicSessionTimeouts)
	return identitycore.SessionExpired(session, now, inactive, absolute)
}

func (store *profileStore) accessPublic(request *http.Request, access tokenAccess) bool {
	if access.resolved {
		return access.sessionFound && access.session.Channel == "public"
	}
	return store.publicSession(request)
}

func (store *profileStore) accessRecentlyAuthenticated(request *http.Request, access tokenAccess, maximumAge time.Duration) bool {
	if access.resolved {
		return access.sessionFound && identitycore.RecentlyStrong(access.session, maximumAge, time.Now())
	}
	return store.recentlyAuthenticated(request, maximumAge)
}
