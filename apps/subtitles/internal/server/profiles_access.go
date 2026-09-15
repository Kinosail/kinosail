package server

import (
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
	key := sessionKey(token)
	if session, found := store.sessions[key]; found && !identitycore.SessionMatchesRequest(session, request) || !found && identitycore.ManagementDeviceKey(request) != "" {
		return viewerProfile{}, false
	}
	session, found, api, apiFound, valid := store.resolveProfileAccess(key, time.Now().Unix())
	if !valid {
		return viewerProfile{}, false
	}
	profileID := session.ProfileID
	if apiFound {
		profileID = api.ProfileID
	}
	return store.authorizedProfile(profileID, key, session, found, api, apiFound)
}

func (store *profileStore) authorizedProfile(profileID, key string, session viewerSession, found bool, api apiKey, apiFound bool) (viewerProfile, bool) {
	profile, exists := profileWithID(store.profiles, profileID)
	if !exists || profile.Disabled || profile.SCIMDeleted {
		return viewerProfile{}, false
	}
	if found && (session.Channel == "public" || session.ManagementDevice != "") && session.ProfileRevision != profile.Revision {
		delete(store.sessions, key)
		_ = store.persist(store.sessionFile, store.sessions)
		return viewerProfile{}, false
	}
	profile.APIKey, profile.Scopes = apiFound, append([]string(nil), api.Scopes...)
	if found && session.Channel == "compatibility" && profile.Owner {
		profile = compatibilityProfile(profile)
	}
	return profile, true
}

func profileWithID(profiles []viewerProfile, id string) (viewerProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return viewerProfile{}, false
}

func (store *profileStore) resolveProfileAccess(key string, now int64) (viewerSession, bool, apiKey, bool, bool) {
	session, found := store.sessions[key]
	api, apiFound := store.apiKeys[key]
	api, apiFound = store.expireAPIKey(key, api, apiFound, now)
	if !apiFound && (!found || store.sessionExpired(session, now)) {
		store.expireSession(key, found)
		return viewerSession{}, false, apiKey{}, false, false
	}
	session = store.touchBrowserSession(key, session, found, now)
	api = store.touchAPIKey(key, api, apiFound, now)
	return session, found, api, apiFound, true
}

func (store *profileStore) expireAPIKey(key string, api apiKey, found bool, now int64) (apiKey, bool) {
	if !found || api.ExpiresAt == 0 || api.ExpiresAt > now {
		return api, found
	}
	delete(store.apiKeys, key)
	_ = store.persist(store.apiFile, store.apiKeys)
	return api, false
}

func (store *profileStore) expireSession(key string, found bool) {
	if !found {
		return
	}
	delete(store.sessions, key)
	_ = store.persist(store.sessionFile, store.sessions)
}

func (store *profileStore) touchBrowserSession(key string, session viewerSession, found bool, now int64) viewerSession {
	if !found || !session.Browser {
		return session
	}
	session.LastSeen = now
	store.sessions[key] = session
	_ = store.persist(store.sessionFile, store.sessions)
	return session
}

func (store *profileStore) touchAPIKey(key string, api apiKey, found bool, now int64) apiKey {
	if !found || api.LastUsed != 0 && api.LastUsed+3600 > now {
		return api
	}
	api.LastUsed = now
	store.apiKeys[key] = api
	_ = store.persist(store.apiFile, store.apiKeys)
	return api
}

func (store *profileStore) sessionExpired(session viewerSession, now int64) bool {
	inactive, absolute := store.sessionTimeouts()
	return identitycore.SessionExpired(session, now, inactive, absolute)
}
