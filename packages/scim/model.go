package scim

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/secure/precis"
)

// Config configures the optional SCIM 2.0 provisioning endpoint.
type Config struct {
	Token          string
	TokenExpiresAt time.Time
}

// Repository commits validated profile operations and their app-specific invariants.
type Repository interface {
	List() []Profile
	Get(string) (Profile, bool)
	Create(ProfileInput) (Profile, error)
	Update(string, ProfileInput, string) (Profile, error)
	Delete(string, string) error
}

// Profile is the protocol view of one provisioned viewer profile.
type Profile struct {
	ID, Name, UserName, ExternalID string
	NameParts                      Name
	Emails                         []Email
	Enterprise                     EnterpriseProfile
	Disabled                       bool
	CreatedAt, UpdatedAt           time.Time
	Revision                       uint64
}

// ProfileInput is one validated create or update operation.
type ProfileInput struct {
	UserName, Name, Formatted, GivenName, FamilyName, ExternalID string
	Emails                                                       []Email
	Enterprise                                                   EnterpriseProfile
	Active                                                       bool
}

// Name contains the persisted SCIM name parts.
type Name struct {
	Formatted, GivenName, FamilyName string
}

// Email contains one persisted SCIM email value.
type Email struct {
	Value, Type string
	Primary     bool
}

// EnterpriseProfile contains the supported enterprise user extension.
type EnterpriseProfile struct {
	EmployeeNumber string  `json:"employeeNumber,omitempty"`
	CostCenter     string  `json:"costCenter,omitempty"`
	Organization   string  `json:"organization,omitempty"`
	Division       string  `json:"division,omitempty"`
	Department     string  `json:"department,omitempty"`
	Manager        Manager `json:"manager,omitempty"`
}

// Manager contains the supported enterprise manager attributes.
type Manager struct {
	Value       string `json:"value,omitempty"`
	Ref         string `json:"$ref,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type (
	scimProfileInput      = ProfileInput
	scimProfileName       = Name
	scimProfileEmail      = Email
	scimEnterpriseProfile = EnterpriseProfile
	scimProfileManager    = Manager
	viewerProfile         = Profile
)

type scimValidationError struct{ detail string }

func (err *scimValidationError) Error() string { return err.detail }

var (
	// ErrProfileNotFound reports a missing provisioned profile.
	ErrProfileNotFound = errors.New("SCIM Viewer Profile was not found")
	// ErrConflict reports a userName or display-name conflict.
	ErrConflict = errors.New("SCIM Viewer Profile conflicts with an existing Profile")
	// ErrPrecondition reports an atomic If-Match conflict.
	ErrPrecondition = errors.New("SCIM Viewer Profile precondition failed")
	// ErrSetupRequired reports that no owner can recover the installation.
	ErrSetupRequired = errors.New("setup must be completed before SCIM provisioning")
)

// NormalizeProfileInput validates and normalizes all writable SCIM fields.
func NormalizeProfileInput(input ProfileInput) (ProfileInput, error) { //nolint:gocognit,cyclop // All SCIM attributes share one semantic validation operation.
	var err error
	input.UserName, err = scimString(input.UserName, 256, true, "userName")
	if err != nil {
		return ProfileInput{}, err
	}
	prepared, prepareErr := precis.UsernameCaseMapped.String(input.UserName)
	if prepareErr != nil {
		return ProfileInput{}, &scimValidationError{detail: "userName is invalid"}
	}
	if input.UserName, err = scimString(prepared, 256, true, "userName"); err != nil {
		return ProfileInput{}, err
	}
	if input.Name, err = scimString(input.Name, 64, true, "displayName"); err != nil {
		return ProfileInput{}, err
	}
	if input.Formatted, err = scimString(input.Formatted, 256, false, "formatted name"); err != nil {
		return ProfileInput{}, err
	}
	if input.GivenName, err = scimString(input.GivenName, 256, false, "given name"); err != nil {
		return ProfileInput{}, err
	}
	if input.FamilyName, err = scimString(input.FamilyName, 256, false, "family name"); err != nil {
		return ProfileInput{}, err
	}
	if input.ExternalID, err = scimString(input.ExternalID, 256, false, "externalId"); err != nil {
		return ProfileInput{}, err
	}
	if input.Enterprise, err = normalizeEnterprise(input.Enterprise); err != nil {
		return ProfileInput{}, err
	}
	if len(input.Emails) > 16 {
		return ProfileInput{}, &scimValidationError{detail: "emails contains too many values"}
	}
	primary := false
	for index := range input.Emails {
		if input.Emails[index].Value, err = scimString(input.Emails[index].Value, 256, true, "email"); err != nil {
			return ProfileInput{}, err
		}
		if input.Emails[index].Type, err = scimString(input.Emails[index].Type, 64, false, "email type"); err != nil {
			return ProfileInput{}, err
		}
		if input.Emails[index].Primary {
			if primary {
				return ProfileInput{}, &scimValidationError{detail: "emails must contain at most one primary value"}
			}
			primary = true
		}
	}
	return input, nil
}

// SameUserName compares prepared SCIM user names.
func SameUserName(left, right string) bool {
	return precis.UsernameCaseMapped.Compare(left, right)
}

func scimString(value string, maximum int, required bool, field string) (string, error) {
	if !utf8.ValidString(value) || len(value) > maximum {
		return "", &scimValidationError{detail: field + " is invalid"}
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", &scimValidationError{detail: field + " is invalid"}
		}
	}
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", &scimValidationError{detail: field + " is invalid"}
	}
	return value, nil
}
