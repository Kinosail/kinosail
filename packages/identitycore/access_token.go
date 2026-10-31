package identitycore

import (
	"context"
	"net/http"
)

type TokenAccess struct {
	Profile                Profile
	Session                Session
	Resolved, SessionFound bool
}

type TokenOutcome int

const (
	TokenDenied TokenOutcome = iota
	TokenAccepted
	TokenSettle
)

type TokenState struct {
	Sessions map[string]Session
	APIKeys  map[string]APIKey
	Profiles []Profile
	Use      *AccessUse
	Expired  func(Session, int64) bool
	Persist  func(ctx context.Context, operation string, apiKeys bool, now int64)
}

func EvaluateToken(state TokenState, request *http.Request, key string, now int64, settle bool) (TokenAccess, TokenOutcome) {
	ctx := request.Context()
	session, found := state.Sessions[key]
	if found && !SessionMatchesRequest(session, request) || !found && ManagementDeviceKey(request) != "" {
		return TokenAccess{}, TokenDenied
	}
	session = state.Use.Session(key, session, now)
	api, apiFound := state.APIKeys[key]
	if apiFound && api.ExpiresAt != 0 && api.ExpiresAt <= now {
		if !settle {
			return TokenAccess{}, TokenSettle
		}
		delete(state.APIKeys, key)
		state.Persist(ctx, "api-key-expire", true, now)
		apiFound = false
	}
	if !apiFound && (!found || state.Expired(session, now)) {
		if found && !settle {
			return TokenAccess{}, TokenSettle
		}
		if found {
			delete(state.Sessions, key)
			state.Persist(ctx, "session-expire", false, now)
		}
		return TokenAccess{}, TokenDenied
	}
	api = state.Use.APIKey(key, api, now)
	profileID := session.ProfileID
	if apiFound {
		profileID = api.ProfileID
	}
	return tokenProfile(ctx, state, now, profileID, key, session, found, api, apiFound, settle)
}

func tokenProfile(ctx context.Context, state TokenState, now int64, profileID, key string, session Session, sessionFound bool, api APIKey, apiFound, settle bool) (TokenAccess, TokenOutcome) {
	profile, exists := FindProfile(state.Profiles, profileID)
	if !exists || profile.Disabled || profile.SCIMDeleted {
		return TokenAccess{}, TokenDenied
	}
	if sessionFound && (session.Channel == "public" || session.ManagementDevice != "") && session.ProfileRevision != profile.Revision {
		if !settle {
			return TokenAccess{}, TokenSettle
		}
		delete(state.Sessions, key)
		state.Persist(ctx, "session-revoke", false, now)
		return TokenAccess{}, TokenDenied
	}
	state.Use.Record(key, session, sessionFound, api, apiFound, now)
	profile.APIKey, profile.Scopes = apiFound, append([]string(nil), api.Scopes...)
	if sessionFound && session.Channel == "compatibility" {
		profile = CompatibilityProfile(profile)
	}
	return TokenAccess{Profile: profile, Session: session, Resolved: true, SessionFound: sessionFound}, TokenAccepted
}

func CompatibilityProfile(profile Profile) Profile {
	if !profile.Owner {
		return profile
	}
	profile.Owner, profile.Downloads, profile.Transcode, profile.Remote = false, true, true, false
	profile.Rating, profile.Libraries = "all", []string{"all"}
	return profile
}
