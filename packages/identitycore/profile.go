package identitycore

import (
	"crypto/rand"
	"errors"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/credentials"
	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/MikeO7/kinosail/packages/scim"
	"github.com/go-webauthn/webauthn/webauthn"
)

type Profile struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Credential     string                    `json:"credential"`
	Owner          bool                      `json:"owner"`
	Downloads      bool                      `json:"downloads,omitempty"`
	Transcode      bool                      `json:"transcode,omitempty"`
	Remote         bool                      `json:"remote,omitempty"`
	Rating         string                    `json:"rating,omitempty"`
	Libraries      []string                  `json:"libraries,omitempty"`
	AccessStart    string                    `json:"accessStart,omitempty"`
	AccessEnd      string                    `json:"accessEnd,omitempty"`
	OIDCSubject    string                    `json:"oidcSubject,omitempty"`
	OIDCIssuer     string                    `json:"oidcIssuer,omitempty"`
	SAMLSubject    string                    `json:"samlSubject,omitempty"`
	SAMLIssuer     string                    `json:"samlIssuer,omitempty"`
	SCIMUserName   string                    `json:"scimUserName,omitempty"`
	SCIMExternalID string                    `json:"scimExternalId,omitempty"`
	SCIMName       scim.Name                 `json:"scimName,omitempty"`
	SCIMEmails     []scim.Email              `json:"scimEmails,omitempty"`
	SCIMEnterprise scim.EnterpriseProfile    `json:"scimEnterprise,omitempty"`
	SCIMManaged    bool                      `json:"scimManaged,omitempty"`
	Disabled       bool                      `json:"disabled,omitempty"`
	SCIMDeleted    bool                      `json:"scimDeleted,omitempty"`
	SCIMCreatedAt  time.Time                 `json:"scimCreatedAt,omitempty"`
	SCIMUpdatedAt  time.Time                 `json:"scimUpdatedAt,omitempty"`
	TOTPSecret     string                    `json:"totpSecret,omitempty"`
	TOTPCounter    int64                     `json:"totpCounter,omitempty"`
	Recovery       []string                  `json:"recoveryCodes,omitempty"`
	APIKey         bool                      `json:"-"`
	Scopes         []string                  `json:"-"`
	Passkeys       []webauthn.Credential     `json:"passkeys,omitempty"`
	PasskeyUsage   map[string]passkeys.Usage `json:"passkeyUsage,omitempty"`
	Revision       uint64                    `json:"revision,omitempty"`
}

type ProfilePolicy struct {
	Downloads, Transcode, Remote   bool
	Rating, AccessStart, AccessEnd string
	Libraries                      []string
}

func NewProfile(name, password string, owner bool) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 || credentials.Validate(password) != nil {
		return Profile{}, errors.New("name and a secure 12-character password are required")
	}
	return newProfile(name, password, owner, credentials.Hash, rand.Text)
}

func newProfile(name, password string, owner bool, hash func(string) (string, error), newID func() string) (Profile, error) {
	credential, err := hash(password)
	if err != nil {
		return Profile{}, err
	}
	return Profile{ID: newID(), Name: name, Credential: credential, Owner: owner, Revision: 1}, nil
}

func CloneProfiles(profiles []Profile) []Profile {
	clone := append([]Profile(nil), profiles...)
	for index := range clone {
		clone[index].Libraries = slices.Clone(clone[index].Libraries)
		clone[index].SCIMEmails = slices.Clone(clone[index].SCIMEmails)
		clone[index].Recovery = slices.Clone(clone[index].Recovery)
		clone[index].Scopes = slices.Clone(clone[index].Scopes)
		clone[index].Passkeys = slices.Clone(clone[index].Passkeys)
		clone[index].PasskeyUsage = maps.Clone(clone[index].PasskeyUsage)
		for key := range clone[index].Passkeys {
			clone[index].Passkeys[key] = passkeys.Clone(clone[index].Passkeys[key])
		}
	}
	return clone
}

func FindProfile(profiles []Profile, id string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return CloneProfiles([]Profile{profile})[0], true
		}
	}
	return Profile{}, false
}

func FederatedProfile(profile Profile) federation.Profile {
	return federation.Profile{
		ID: profile.ID, SCIMExternalID: profile.SCIMExternalID,
		OIDC:        federation.Identity{Issuer: profile.OIDCIssuer, Subject: profile.OIDCSubject},
		SAML:        federation.Identity{Issuer: profile.SAMLIssuer, Subject: profile.SAMLSubject},
		SCIMManaged: profile.SCIMManaged, Disabled: profile.Disabled, SCIMDeleted: profile.SCIMDeleted,
	}
}

func ApplyFederatedIdentity(profile *Profile, protocol federation.Protocol, identity federation.Identity) {
	if protocol == federation.SAMLProtocol {
		profile.SAMLIssuer, profile.SAMLSubject = identity.Issuer, identity.Subject
	} else {
		profile.OIDCIssuer, profile.OIDCSubject = identity.Issuer, identity.Subject
	}
}

func (profile Profile) Secured() bool { return Secured(profile.TOTPSecret, len(profile.Passkeys)) }

func (profile Profile) Permits(scope string, allowed bool) bool {
	return allowed && (!profile.APIKey || slices.Contains(profile.Scopes, scope))
}

func (profile Profile) AllowsAPI(pattern string, routes APIRoutes) bool {
	return AllowsAPI(profile.Scopes, pattern, routes)
}

func (profile Profile) Allowed(public bool, now time.Time) bool {
	if profile.Owner {
		return true
	}
	if public && !profile.Remote {
		return false
	}
	if profile.AccessStart == "" || profile.AccessEnd == "" {
		return true
	}
	start, _ := time.Parse("15:04", profile.AccessStart)
	end, _ := time.Parse("15:04", profile.AccessEnd)
	minute, first, last := now.Hour()*60+now.Minute(), start.Hour()*60+start.Minute(), end.Hour()*60+end.Minute()
	if first <= last {
		return minute >= first && minute < last
	}
	return minute >= first || minute < last
}

func ProfilePolicyFromRequest(request *http.Request) ProfilePolicy {
	return ProfilePolicy{
		Downloads:   request.FormValue("downloads") == "true",
		Transcode:   request.FormValue("transcode") == "true",
		Remote:      request.FormValue("remote") == "true",
		Rating:      request.FormValue("rating"),
		Libraries:   splitLibraries(request.FormValue("libraries")),
		AccessStart: strings.TrimSpace(request.FormValue("start")),
		AccessEnd:   strings.TrimSpace(request.FormValue("end")),
	}
}

func (policy ProfilePolicy) Valid() error {
	if (policy.AccessStart == "") != (policy.AccessEnd == "") {
		return errors.New("both viewing schedule times are required")
	}
	for _, value := range []string{policy.AccessStart, policy.AccessEnd} {
		if value != "" {
			if _, err := time.Parse("15:04", value); err != nil {
				return errors.New("viewing schedule must use HH:MM")
			}
		}
	}
	return nil
}

func ApplyProfilePolicy(profile *Profile, policy ProfilePolicy) {
	profile.Downloads, profile.Transcode, profile.Remote = policy.Downloads, policy.Transcode, policy.Remote
	profile.Rating, profile.Libraries = normalizeRating(policy.Rating), slices.Clone(policy.Libraries)
	profile.AccessStart, profile.AccessEnd = policy.AccessStart, policy.AccessEnd
}

func normalizeRating(rating string) string {
	if rating == "teen" || rating == "all" {
		return rating
	}
	return "family"
}

func splitLibraries(value string) []string {
	unique := make(map[string]bool)
	for _, name := range strings.Split(value, ",") {
		if name = strings.TrimSpace(name); name != "" {
			unique[name] = true
		}
	}
	libraries := make([]string, 0, len(unique))
	for name := range unique {
		libraries = append(libraries, name)
	}
	sort.Strings(libraries)
	return libraries
}

type ProfileBatchStore interface {
	SaveJSONBatch(map[string]any) error
}

type ProfilePersistence struct {
	Database                          ProfileBatchStore
	Persist                           func(string, any) error
	ProfileFile, SessionFile, APIFile string
}

func NewProfilePersistence(database ProfileBatchStore, databaseEnabled bool, persist func(string, any) error, profileFile, sessionFile, apiFile string) ProfilePersistence {
	persistence := ProfilePersistence{Persist: persist, ProfileFile: profileFile, SessionFile: sessionFile, APIFile: apiFile}
	if databaseEnabled {
		persistence.Database = database
	}
	return persistence
}

func (persistence ProfilePersistence) Save(profiles []Profile) error {
	if persistence.ProfileFile == "" || persistence.Persist == nil {
		return errors.New("profile storage is not configured")
	}
	return persistence.Persist(persistence.ProfileFile, profiles)
}

func (persistence ProfilePersistence) SaveRelated(profiles []Profile, sessions map[string]Session, keys map[string]APIKey) error {
	if persistence.Database != nil {
		return persistence.Database.SaveJSONBatch(map[string]any{"profiles.json": profiles, "sessions.json": sessions, "api_keys.json": keys})
	}
	if persistence.ProfileFile == "" || persistence.SessionFile == "" || persistence.APIFile == "" || persistence.Persist == nil {
		return errors.New("profile storage is not configured")
	}
	if err := persistence.Persist(persistence.SessionFile, sessions); err != nil {
		return err
	}
	if err := persistence.Persist(persistence.APIFile, keys); err != nil {
		return err
	}
	return persistence.Persist(persistence.ProfileFile, profiles)
}
