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
	session, found := store.sessions[key]
	if found && !identitycore.SessionMatchesRequest(session, request) || !found && identitycore.ManagementDeviceKey(request) != "" {
		return viewerProfile{}, false
	}
	api, apiFound := store.apiKeys[key]
	api, apiFound = store.refreshAPIKey(ctx, key, api, apiFound, now)
	if !apiFound && (!found || store.sessionExpired(session, now)) {
		if found {
			delete(store.sessions, key)
			store.saveAccessState(ctx, "session-expire", store.sessionFile, now)
		}
		return viewerProfile{}, false
	}
	store.recordTokenUse(ctx, key, session, found, api, apiFound, now)
	profileID := session.ProfileID
	if apiFound {
		profileID = api.ProfileID
	}
	return store.profileForToken(ctx, now, profileID, key, session, found, api, apiFound)
}

func (store *profileStore) profileForToken(ctx context.Context, now int64, profileID, key string, session viewerSession, sessionFound bool, api apiKey, apiFound bool) (viewerProfile, bool) {
	for _, profile := range store.profiles {
		if profile.ID == profileID {
			if !store.validProfileToken(ctx, now, profile, key, session, sessionFound) {
				return viewerProfile{}, false
			}
			profile.APIKey, profile.Scopes = apiFound, append([]string(nil), api.Scopes...)
			if sessionFound && session.Channel == "compatibility" && profile.Owner {
				profile = compatibilityProfile(profile)
			}
			return profile, true
		}
	}
	return viewerProfile{}, false
}

func (store *profileStore) refreshAPIKey(ctx context.Context, key string, api apiKey, found bool, now int64) (apiKey, bool) {
	if found && api.ExpiresAt != 0 && api.ExpiresAt <= now {
		delete(store.apiKeys, key)
		store.saveAccessState(ctx, "api-key-expire", store.apiFile, now)
		return api, false
	}
	return api, found
}

func (store *profileStore) recordTokenUse(ctx context.Context, key string, session viewerSession, sessionFound bool, api apiKey, apiFound bool, now int64) {
	if sessionFound && session.Browser && session.LastSeen+sessionUseWriteSeconds <= now {
		session.LastSeen = now
		store.sessions[key] = session
		store.saveAccessState(ctx, "session-use", store.sessionFile, now)
	}
	if apiFound && (api.LastUsed == 0 || api.LastUsed+apiKeyUseWriteSeconds <= now) {
		api.LastUsed = now
		store.apiKeys[key] = api
		store.saveAccessState(ctx, "api-key-use", store.apiFile, now)
	}
}

func (store *profileStore) sessionExpired(session viewerSession, now int64) bool {
	inactive, absolute := store.sessionModule().TimeoutsFor(session)
	return identitycore.SessionExpired(session, now, inactive, absolute)
}

func (store *profileStore) validProfileToken(ctx context.Context, now int64, profile viewerProfile, key string, session viewerSession, sessionFound bool) bool {
	if profile.Disabled || profile.SCIMDeleted {
		return false
	}
	if sessionFound && (session.Channel == "public" || session.ManagementDevice != "") && session.ProfileRevision != profile.Revision {
		delete(store.sessions, key)
		store.saveAccessState(ctx, "session-revoke", store.sessionFile, now)
		return false
	}
	return true
}
