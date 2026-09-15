package server

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func (store *profileStore) profile(request *http.Request) (viewerProfile, bool) { //nolint:cyclop,gocognit,funlen // Session and compatibility-token resolution share one identity boundary.
	token := sessionToken(request)
	if token == "" {
		return viewerProfile{}, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	key := sessionKey(token)
	session, found := store.sessions[key]
	if found && !identitycore.SessionMatchesRequest(session, request) || !found && identitycore.ManagementDeviceKey(request) != "" {
		return viewerProfile{}, false
	}
	api, apiFound := store.apiKeys[key]
	now := time.Now().Unix()
	api, apiFound = store.refreshAPIKey(key, api, apiFound, now)
	if !apiFound && (!found || store.sessionExpired(session, now)) {
		if found {
			delete(store.sessions, key)
			_ = store.persist(store.sessionFile, store.sessions)
		}
		return viewerProfile{}, false
	}
	store.recordTokenUse(key, session, found, api, apiFound, now)
	profileID := session.ProfileID
	if apiFound {
		profileID = api.ProfileID
	}
	return store.profileForToken(profileID, key, session, found, api, apiFound)
}

func (store *profileStore) profileForToken(profileID, key string, session viewerSession, sessionFound bool, api apiKey, apiFound bool) (viewerProfile, bool) {
	for _, profile := range store.profiles {
		if profile.ID == profileID {
			if !store.validProfileToken(profile, key, session, sessionFound) {
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

func (store *profileStore) refreshAPIKey(key string, api apiKey, found bool, now int64) (apiKey, bool) {
	if found && api.ExpiresAt != 0 && api.ExpiresAt <= now {
		delete(store.apiKeys, key)
		_ = store.persist(store.apiFile, store.apiKeys)
		return api, false
	}
	return api, found
}

func (store *profileStore) recordTokenUse(key string, session viewerSession, sessionFound bool, api apiKey, apiFound bool, now int64) {
	if sessionFound && session.Browser {
		session.LastSeen = now
		store.sessions[key] = session
		_ = store.persist(store.sessionFile, store.sessions)
	}
	if apiFound && (api.LastUsed == 0 || api.LastUsed+3600 <= now) {
		api.LastUsed = now
		store.apiKeys[key] = api
		_ = store.persist(store.apiFile, store.apiKeys)
	}
}

func (store *profileStore) sessionExpired(session viewerSession, now int64) bool {
	inactive, absolute := store.sessionTimeouts()
	return identitycore.SessionExpired(session, now, inactive, absolute)
}

func (store *profileStore) validProfileToken(profile viewerProfile, key string, session viewerSession, sessionFound bool) bool {
	if profile.Disabled || profile.SCIMDeleted {
		return false
	}
	if sessionFound && (session.Channel == "public" || session.ManagementDevice != "") && session.ProfileRevision != profile.Revision {
		delete(store.sessions, key)
		_ = store.persist(store.sessionFile, store.sessions)
		return false
	}
	return true
}
