package identitycore

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/scim"
)

// ValidateProfiles rejects ambiguous or malformed persisted identity state.
func ValidateProfiles(profiles []Profile) error {
	ids := make(map[string]struct{}, len(profiles))
	userNames := make(map[string]struct{})
	oidcIdentities, samlIdentities := make(map[string]struct{}), make(map[string]struct{})
	for _, profile := range profiles {
		if _, found := ids[profile.ID]; found {
			return errors.New("profile contains duplicate ID")
		}
		ids[profile.ID] = struct{}{}
		userName, err := validateProfile(profile, oidcIdentities, samlIdentities)
		if err != nil {
			return err
		}
		if userName == "" {
			continue
		}
		if _, found := userNames[userName]; found {
			return errors.New("profile contains duplicate SCIM userName")
		}
		userNames[userName] = struct{}{}
	}
	return nil
}

func validateProfile(profile Profile, oidcIdentities, samlIdentities map[string]struct{}) (string, error) {
	if profile.ID == "" {
		return "", errors.New("profile contains invalid SCIM state")
	}
	if err := validateFederatedProfileIdentity(profile.OIDCIssuer, profile.OIDCSubject, oidcIdentities); err != nil {
		return "", err
	}
	if err := validateFederatedProfileIdentity(profile.SAMLIssuer, profile.SAMLSubject, samlIdentities); err != nil {
		return "", err
	}
	if !profile.SCIMManaged {
		if !validUnmanagedSCIMProfile(profile) {
			return "", errors.New("profile contains invalid SCIM state")
		}
		return "", nil
	}
	if !validManagedSCIMProfile(profile) {
		return "", errors.New("profile contains invalid SCIM state")
	}
	input, err := scim.NormalizeProfileInput(scim.ProfileInput{UserName: profile.SCIMUserName, Name: profile.Name, Formatted: profile.SCIMName.Formatted, GivenName: profile.SCIMName.GivenName, FamilyName: profile.SCIMName.FamilyName, ExternalID: profile.SCIMExternalID, Emails: append([]scim.Email(nil), profile.SCIMEmails...), Enterprise: profile.SCIMEnterprise, Active: !profile.Disabled})
	if err != nil || !samePersistedSCIMProfile(input, profile) {
		return "", errors.New("profile contains invalid SCIM state")
	}
	return input.UserName, nil
}

func validUnmanagedSCIMProfile(profile Profile) bool {
	return profile.SCIMUserName == "" && profile.SCIMExternalID == "" && !profile.SCIMDeleted && profile.SCIMCreatedAt.IsZero() && profile.SCIMUpdatedAt.IsZero() && profile.SCIMName == (scim.Name{}) && len(profile.SCIMEmails) == 0 && profile.SCIMEnterprise == (scim.EnterpriseProfile{})
}

func validManagedSCIMProfile(profile Profile) bool { //nolint:cyclop // The score of 11 remains below the repository ceiling of 22 for one complete invariant predicate.
	credentialsValid := !profile.Owner && profile.Credential == "" && profile.TOTPSecret == "" && len(profile.Recovery) == 0 && len(profile.Passkeys) == 0
	identityValid := profile.SCIMUserName != "" && !profile.SCIMCreatedAt.IsZero() && !profile.SCIMUpdatedAt.IsZero() && !profile.SCIMUpdatedAt.Before(profile.SCIMCreatedAt) && (!profile.SCIMDeleted || profile.Disabled)
	return credentialsValid && identityValid
}

func samePersistedSCIMProfile(input scim.ProfileInput, profile Profile) bool {
	return input.UserName == profile.SCIMUserName && input.Name == profile.Name && input.Formatted == profile.SCIMName.Formatted && input.GivenName == profile.SCIMName.GivenName && input.FamilyName == profile.SCIMName.FamilyName && input.ExternalID == profile.SCIMExternalID && equalSCIMEmails(input.Emails, profile.SCIMEmails) && input.Enterprise == profile.SCIMEnterprise
}

func validateFederatedProfileIdentity(issuer, subject string, identities map[string]struct{}) error {
	if (issuer == "") != (subject == "") {
		return errors.New("profile contains incomplete federated identity")
	}
	if issuer == "" {
		return nil
	}
	validIssuer, issuerErr := validIdentityString(issuer, 2048)
	validSubject, subjectErr := validIdentityString(subject, 1024)
	if issuerErr != nil || subjectErr != nil || validIssuer != issuer || validSubject != subject {
		return errors.New("profile contains invalid federated identity")
	}
	key := issuer + "\x00" + subject
	if _, found := identities[key]; found {
		return errors.New("profile contains duplicate federated identity")
	}
	identities[key] = struct{}{}
	return nil
}

func equalSCIMEmails(left, right []scim.Email) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validIdentityString(value string, maximum int) (string, error) {
	if !utf8.ValidString(value) || len(value) > maximum {
		return "", errors.New("identity is invalid")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", errors.New("identity is invalid")
		}
	}
	return strings.TrimSpace(value), nil
}
