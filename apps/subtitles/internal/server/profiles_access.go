package server

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

type tokenAccess = identitycore.TokenAccess

func (store *profileStore) access(request *http.Request) (tokenAccess, bool) {
	token := sessionToken(request)
	if token == "" {
		return tokenAccess{}, false
	}
	key, now := sessionKey(token), time.Now().Unix()
	store.mu.RLock()
	access, outcome := store.evaluateToken(request, key, now, false)
	store.mu.RUnlock()
	if outcome == identitycore.TokenSettle {
		store.mu.Lock()
		defer store.mu.Unlock()
		access, outcome = store.evaluateToken(request, key, now, true)
	}
	return access, outcome == identitycore.TokenAccepted
}

func (store *profileStore) evaluateToken(request *http.Request, key string, now int64, settle bool) (tokenAccess, identitycore.TokenOutcome) {
	return identitycore.EvaluateToken(identitycore.TokenState{
		Sessions: store.sessions, APIKeys: store.apiKeys, Profiles: store.profiles, Use: &store.accessUse,
		Expired: store.sessionExpired, Persist: store.persistAccess,
	}, request, key, now, settle)
}

func (store *profileStore) persistAccess(ctx context.Context, operation string, apiKeys bool, now int64) {
	file := store.sessionFile
	if apiKeys {
		file = store.apiFile
	}
	store.saveAccessState(ctx, operation, file, now)
}

func (store *profileStore) sessionExpired(session viewerSession, now int64) bool {
	inactive, absolute := identitycore.SessionTimeouts(session, store.sessionTimeouts, store.publicSessionTimeouts)
	return identitycore.SessionExpired(session, now, inactive, absolute)
}

func (store *profileStore) accessPublic(request *http.Request, access tokenAccess) bool {
	if access.Resolved {
		return access.SessionFound && access.Session.Channel == "public"
	}
	return store.publicSession(request)
}

func (store *profileStore) accessRecentlyAuthenticated(request *http.Request, access tokenAccess, maximumAge time.Duration) bool {
	if access.Resolved {
		return access.SessionFound && identitycore.RecentlyStrong(access.Session, maximumAge, time.Now())
	}
	return store.recentlyAuthenticated(request, maximumAge)
}
