package identitycore

import (
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/scim"
)

// SCIMProfiles returns active provisioned profiles.
func (store ProfileStore) SCIMProfiles() []Profile {
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	result := make([]Profile, 0)
	for _, profile := range *store.config.Profiles {
		if profile.SCIMManaged && !profile.SCIMDeleted {
			result = append(result, profile)
		}
	}
	return result
}

// SCIMProfile returns one active provisioned profile.
func (store ProfileStore) SCIMProfile(id string) (Profile, bool) {
	store.config.Mutex.RLock()
	defer store.config.Mutex.RUnlock()
	for _, profile := range *store.config.Profiles {
		if profile.ID == id && profile.SCIMManaged && !profile.SCIMDeleted {
			return profile, true
		}
	}
	return Profile{}, false
}

// CreateSCIMProfile atomically provisions or rehydrates a profile.
func (store ProfileStore) CreateSCIMProfile(input scim.ProfileInput) (Profile, error) { //nolint:cyclop,gocognit // Scores of 16 and 17 remain below the repository ceiling of 22 for one atomic create transaction.
	input, err := scim.NormalizeProfileInput(input)
	if err != nil {
		return Profile{}, err
	}
	now := store.config.Now().UTC()
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	owners := false
	for _, profile := range *store.config.Profiles {
		owners = owners || profile.Owner
	}
	if !owners {
		return Profile{}, scim.ErrSetupRequired
	}
	profiles := CloneProfiles(*store.config.Profiles)
	rehydrate := -1
	for index, existing := range profiles {
		if existing.SCIMManaged && existing.SCIMDeleted && scim.SameUserName(existing.SCIMUserName, input.UserName) {
			if rehydrate >= 0 || input.ExternalID == "" || existing.SCIMExternalID != input.ExternalID {
				return Profile{}, scim.ErrConflict
			}
			rehydrate = index
			continue
		}
		if strings.EqualFold(existing.Name, input.Name) || existing.SCIMManaged && !existing.SCIMDeleted && scim.SameUserName(existing.SCIMUserName, input.UserName) {
			return Profile{}, scim.ErrConflict
		}
	}
	if rehydrate >= 0 {
		return store.rehydrateSCIMProfile(profiles, rehydrate, input, now)
	}
	profile := Profile{ID: newSCIMID(profiles, store.config.NewID), Name: input.Name, SCIMUserName: input.UserName, SCIMExternalID: input.ExternalID, SCIMName: scim.Name{Formatted: input.Formatted, GivenName: input.GivenName, FamilyName: input.FamilyName}, SCIMEmails: append([]scim.Email(nil), input.Emails...), SCIMEnterprise: input.Enterprise, SCIMManaged: true, Disabled: !input.Active, Rating: "family", Revision: 1, SCIMCreatedAt: now, SCIMUpdatedAt: now}
	profiles = append(profiles, profile)
	if err := store.config.Persistence.SaveRelated(profiles, *store.config.Sessions, *store.config.APIKeys); err != nil {
		return Profile{}, err
	}
	*store.config.Profiles = profiles
	return profile, nil
}

func (store ProfileStore) rehydrateSCIMProfile(profiles []Profile, index int, input scim.ProfileInput, now time.Time) (Profile, error) {
	existing := profiles[index]
	existing.Name, existing.SCIMUserName, existing.SCIMExternalID = input.Name, input.UserName, input.ExternalID
	existing.SCIMName, existing.SCIMEmails, existing.SCIMEnterprise = scim.Name{Formatted: input.Formatted, GivenName: input.GivenName, FamilyName: input.FamilyName}, append([]scim.Email(nil), input.Emails...), input.Enterprise
	existing.Credential, existing.OIDCIssuer, existing.OIDCSubject, existing.SAMLIssuer, existing.SAMLSubject, existing.TOTPSecret, existing.Recovery, existing.Passkeys, existing.PasskeyUsage = "", "", "", "", "", "", nil, nil, nil
	existing.Disabled, existing.SCIMDeleted, existing.SCIMUpdatedAt = !input.Active, false, now
	existing.Revision++
	profiles[index] = existing
	sessions := WithoutProfile(*store.config.Sessions, existing.ID, false)
	if err := store.config.Persistence.SaveRelated(profiles, sessions, *store.config.APIKeys); err != nil {
		return Profile{}, err
	}
	*store.config.Profiles, *store.config.Sessions = profiles, sessions
	return existing, nil
}

func newSCIMID(profiles []Profile, next func() string) string {
	for {
		candidate := next()
		if _, found := FindProfile(profiles, candidate); !found {
			return candidate
		}
	}
}

// UpdateSCIMProfile atomically replaces a provisioned profile.
func (store ProfileStore) UpdateSCIMProfile(id string, input scim.ProfileInput, expected string) (Profile, error) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for one atomic update transaction.
	input, err := scim.NormalizeProfileInput(input)
	if err != nil {
		return Profile{}, err
	}
	now := store.config.Now().UTC()
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	profiles := CloneProfiles(*store.config.Profiles)
	for index, existing := range profiles {
		if existing.ID != id || !existing.SCIMManaged || existing.SCIMDeleted {
			continue
		}
		if expected != "" && !scim.ETagMatches(expected, `W/"`+strconv.FormatUint(existing.Revision, 10)+`"`) {
			return Profile{}, scim.ErrPrecondition
		}
		if conflictingSCIMProfile(profiles, id, input) {
			return Profile{}, scim.ErrConflict
		}
		wasDisabled := existing.Disabled
		identityChanged := existing.SCIMExternalID != input.ExternalID
		if identityChanged {
			existing.Credential, existing.TOTPSecret, existing.Recovery, existing.Passkeys, existing.PasskeyUsage = "", "", nil, nil, nil
			existing.OIDCIssuer, existing.OIDCSubject, existing.SAMLIssuer, existing.SAMLSubject = "", "", "", ""
		}
		existing.Name, existing.SCIMUserName, existing.SCIMExternalID = input.Name, input.UserName, input.ExternalID
		existing.SCIMName, existing.SCIMEmails, existing.SCIMEnterprise = scim.Name{Formatted: input.Formatted, GivenName: input.GivenName, FamilyName: input.FamilyName}, append([]scim.Email(nil), input.Emails...), input.Enterprise
		existing.Disabled, existing.Revision, existing.SCIMUpdatedAt = !input.Active, existing.Revision+1, now
		profiles[index] = existing
		sessions := CloneSessions(*store.config.Sessions)
		if wasDisabled || !input.Active || identityChanged {
			sessions = WithoutProfile(sessions, id, false)
		}
		keys := CloneAPIKeys(*store.config.APIKeys)
		if identityChanged {
			for key, value := range keys {
				if value.ProfileID == id {
					delete(keys, key)
				}
			}
		}
		if err := store.config.Persistence.SaveRelated(profiles, sessions, keys); err != nil {
			return Profile{}, err
		}
		*store.config.Profiles, *store.config.Sessions, *store.config.APIKeys = profiles, sessions, keys
		return existing, nil
	}
	return Profile{}, scim.ErrProfileNotFound
}

func conflictingSCIMProfile(profiles []Profile, id string, input scim.ProfileInput) bool {
	for _, candidate := range profiles {
		if candidate.ID == id {
			continue
		}
		if strings.EqualFold(candidate.Name, input.Name) || candidate.SCIMManaged && !candidate.SCIMDeleted && scim.SameUserName(candidate.SCIMUserName, input.UserName) {
			return true
		}
	}
	return false
}

// DeleteSCIMProfile atomically deprovisions a profile.
func (store ProfileStore) DeleteSCIMProfile(id, expected string) error {
	now := store.config.Now().UTC()
	store.config.Mutex.Lock()
	defer store.config.Mutex.Unlock()
	profiles := CloneProfiles(*store.config.Profiles)
	for index, profile := range profiles {
		if profile.ID != id || !profile.SCIMManaged || profile.SCIMDeleted {
			continue
		}
		if expected != "" && !scim.ETagMatches(expected, `W/"`+strconv.FormatUint(profile.Revision, 10)+`"`) {
			return scim.ErrPrecondition
		}
		profile.Disabled, profile.SCIMDeleted, profile.Revision, profile.SCIMUpdatedAt = true, true, profile.Revision+1, now
		profiles[index] = profile
		sessions := WithoutProfile(*store.config.Sessions, id, false)
		if err := store.config.Persistence.SaveRelated(profiles, sessions, *store.config.APIKeys); err != nil {
			return err
		}
		*store.config.Profiles, *store.config.Sessions = profiles, sessions
		return nil
	}
	return scim.ErrProfileNotFound
}
