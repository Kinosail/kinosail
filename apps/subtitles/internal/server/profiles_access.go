package server

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func (store *profileStore) profile(request *http.Request) (viewerProfile, bool) {
	token := sessionToken(request)
	if token == "" {
		return viewerProfile{}, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	ctx, now := request.Context(), time.Now().Unix()
	store.retryAccessState(ctx, now)
	key := sessionKey(token)
	if session, found := store.sessions[key]; found && !identitycore.SessionMatchesRequest(session, request) || !found && identitycore.ManagementDeviceKey(request) != "" {
		return viewerProfile{}, false
	}
	session, found, api, apiFound, valid := store.resolveProfileAccess(ctx, key, now)
	if !valid {
		return viewerProfile{}, false
	}
	profileID := session.ProfileID
	if apiFound {
		profileID = api.ProfileID
	}
	return store.authorizedProfile(ctx, now, profileID, key, session, found, api, apiFound)
}

func (store *profileStore) authorizedProfile(ctx context.Context, now int64, profileID, key string, session viewerSession, found bool, api apiKey, apiFound bool) (viewerProfile, bool) {
	profile, exists := profileWithID(store.profiles, profileID)
	if !exists || profile.Disabled || profile.SCIMDeleted {
		return viewerProfile{}, false
	}
	if found && store.revokeStaleSession(ctx, now, key, session, profile) {
		return viewerProfile{}, false
	}
	profile.APIKey, profile.Scopes = apiFound, append([]string(nil), api.Scopes...)
	if found && session.Channel == "compatibility" && profile.Owner {
		profile = compatibilityProfile(profile)
	}
	return profile, true
}

func (store *profileStore) revokeStaleSession(ctx context.Context, now int64, key string, session viewerSession, profile viewerProfile) bool {
	if session.Channel != "public" && session.ManagementDevice == "" || session.ProfileRevision == profile.Revision {
		return false
	}
	delete(store.sessions, key)
	store.saveAccessState(ctx, "session-revoke", store.sessionFile, now)
	return true
}

func profileWithID(profiles []viewerProfile, id string) (viewerProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return viewerProfile{}, false
}

func (store *profileStore) resolveProfileAccess(ctx context.Context, key string, now int64) (viewerSession, bool, apiKey, bool, bool) {
	session, found := store.sessions[key]
	api, apiFound := store.apiKeys[key]
	api, apiFound = store.expireAPIKey(ctx, key, api, apiFound, now)
	if !apiFound && (!found || store.sessionExpired(session, now)) {
		store.expireSession(ctx, key, found, now)
		return viewerSession{}, false, apiKey{}, false, false
	}
	session = store.touchBrowserSession(ctx, key, session, found, now)
	api = store.touchAPIKey(ctx, key, api, apiFound, now)
	return session, found, api, apiFound, true
}

func (store *profileStore) expireAPIKey(ctx context.Context, key string, api apiKey, found bool, now int64) (apiKey, bool) {
	if !found || api.ExpiresAt == 0 || api.ExpiresAt > now {
		return api, found
	}
	delete(store.apiKeys, key)
	store.saveAccessState(ctx, "api-key-expire", store.apiFile, now)
	return api, false
}

func (store *profileStore) expireSession(ctx context.Context, key string, found bool, now int64) {
	if !found {
		return
	}
	delete(store.sessions, key)
	store.saveAccessState(ctx, "session-expire", store.sessionFile, now)
}

func (store *profileStore) touchBrowserSession(ctx context.Context, key string, session viewerSession, found bool, now int64) viewerSession {
	if !found || !session.Browser || session.LastSeen+sessionUseWriteSeconds > now {
		return session
	}
	session.LastSeen = now
	store.sessions[key] = session
	store.saveAccessState(ctx, "session-use", store.sessionFile, now)
	return session
}

func (store *profileStore) touchAPIKey(ctx context.Context, key string, api apiKey, found bool, now int64) apiKey {
	if !found || api.LastUsed != 0 && api.LastUsed+apiKeyUseWriteSeconds > now {
		return api
	}
	api.LastUsed = now
	store.apiKeys[key] = api
	store.saveAccessState(ctx, "api-key-use", store.apiFile, now)
	return api
}

func (store *profileStore) sessionExpired(session viewerSession, now int64) bool {
	inactive, absolute := store.sessionModule().TimeoutsFor(session)
	return identitycore.SessionExpired(session, now, inactive, absolute)
}
