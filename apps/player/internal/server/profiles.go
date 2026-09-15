package server

import (
	"sync"
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/identitycore"
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
)

type (
	viewerProfile = identitycore.Profile
	profilePolicy = identitycore.ProfilePolicy
	passkeyUsage  = sharedpasskeys.Usage
)

type profileStore struct {
	mu                         sync.RWMutex
	file, sessionFile, apiFile string
	profiles                   []viewerProfile
	sessions                   map[string]viewerSession
	apiKeys                    map[string]apiKey
	err                        error
	persist                    func(string, any) error
	database                   *database.Store
	sessionTimeouts            func() (time.Duration, time.Duration)
}

func newProfileStore(dataDir string, databases ...*database.Store) *profileStore {
	stateDB := configuredDatabase(databases)
	state := identitycore.LoadProfileState(dataDir,
		func(path string, target any) (bool, error) { return loadState(stateDB, path, target) },
		func(path string) (map[string]viewerSession, error) { return loadSessions(path, stateDB) },
		func(path string) (map[string]apiKey, error) { return loadAPIKeys(path, stateDB) },
	)
	return &profileStore{
		file: state.ProfileFile, sessionFile: state.SessionFile, apiFile: state.APIFile,
		profiles: state.Profiles, sessions: state.Sessions, apiKeys: state.APIKeys, err: state.Err,
		persist: statePersistence(stateDB), database: stateDB,
		sessionTimeouts: func() (time.Duration, time.Duration) { return defaultSessionInactive, defaultSessionAbsolute },
	}
}

func (store *profileStore) hasProfiles() bool {
	return store.profileModule().HasProfiles()
}

func (store *profileStore) authenticate(name, password string) (viewerProfile, bool) {
	return store.profileModule().Authenticate(name, password)
}

func (store *profileStore) addProfile(name, password string, owner bool, policy profilePolicy) (string, error) {
	return store.profileModule().AddProfile(name, password, owner, policy)
}

func (store *profileStore) setProfile(id string, owner bool, policy profilePolicy) error {
	return store.profileModule().SetProfile(id, owner, policy)
}

func (store *profileStore) removeProfile(id string) error {
	return store.profileModule().RemoveProfile(id)
}
