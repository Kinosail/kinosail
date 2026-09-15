package identitycore

import "path/filepath"

// ProfileState is the canonical loaded profile document set.
type ProfileState struct {
	ProfileFile, SessionFile, APIFile string
	Profiles                          []Profile
	Sessions                          map[string]Session
	APIKeys                           map[string]APIKey
	Err                               error
}

// LoadProfileState loads Player's related profile documents in dependency order.
func LoadProfileState(
	dataDir string,
	loadProfiles func(string, any) (bool, error),
	loadSessions func(string) (map[string]Session, error),
	loadAPIKeys func(string) (map[string]APIKey, error),
) ProfileState {
	state := ProfileState{Sessions: make(map[string]Session), APIKeys: make(map[string]APIKey)}
	if dataDir == "" {
		return state
	}
	state.ProfileFile = filepath.Join(dataDir, "profiles.json")
	state.SessionFile = filepath.Join(dataDir, "sessions.json")
	state.APIFile = filepath.Join(dataDir, "api_keys.json")
	_, state.Err = loadProfiles(state.ProfileFile, &state.Profiles)
	if state.Err == nil {
		state.Err = ValidateProfiles(state.Profiles)
	}
	if state.Err == nil {
		state.Sessions, state.Err = loadSessions(state.SessionFile)
	}
	if state.Err == nil {
		state.APIKeys, state.Err = loadAPIKeys(state.APIFile)
	}
	return state
}
