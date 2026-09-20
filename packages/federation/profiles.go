package federation

import "errors"

// Protocol selects one federated identity slot and its product-facing errors.
type Protocol string

const (
	OIDCProtocol Protocol = "OIDC"
	SAMLProtocol Protocol = "SAML"
)

// Profile describes the federation and provisioning fields shared by every app profile.
type Profile struct {
	ID, SCIMExternalID    string
	OIDC, SAML            Identity
	SCIMManaged           bool
	Disabled, SCIMDeleted bool
}

// Profiles owns atomic federation lookup, linking, provisioning, and persistence policy.
type Profiles[T any] struct {
	Lock, Unlock func()
	Clone        func() []T
	Persist      func([]T) error
	Commit       func([]T)
	Inspect      func(T) Profile
	Apply        func(*T, Protocol, Identity)
	// SessionActive is called while the profile lock is held.
	SessionActive func(string, string) bool
}

// Find returns the enabled profile linked to one verified identity.
func (profiles Profiles[T]) Find(protocol Protocol, identity Identity) (T, bool) {
	var zero T
	if validateIdentity(protocol, identity) != nil {
		return zero, false
	}
	profiles.Lock()
	defer profiles.Unlock()
	for _, candidate := range profiles.Clone() {
		fields := profiles.Inspect(candidate)
		if !fields.Disabled && !fields.SCIMDeleted && (!fields.SCIMManaged || fields.SCIMExternalID == identity.Subject) && profileIdentity(fields, protocol) == identity {
			return candidate, true
		}
	}
	return zero, false
}

// FindByID returns one profile without changing its state.
func (profiles Profiles[T]) FindByID(id string) (T, bool) {
	profiles.Lock()
	defer profiles.Unlock()
	for _, candidate := range profiles.Clone() {
		if profiles.Inspect(candidate).ID == id {
			return candidate, true
		}
	}
	var zero T
	return zero, false
}

// AutoLinkSCIM links one uniquely matching active SCIM profile and persists it atomically.
func (profiles Profiles[T]) AutoLinkSCIM(protocol Protocol, identity Identity) (T, bool, error) {
	var zero T
	if identity == (Identity{}) {
		return zero, false, nil
	}
	if err := validateIdentity(protocol, identity); err != nil {
		return zero, false, err
	}
	profiles.Lock()
	defer profiles.Unlock()
	values := profiles.Clone()
	match, unique := uniqueSCIMMatch(values, identity.Subject, profiles.Inspect)
	if !unique || match < 0 || identityLinkedElsewhere(values, match, protocol, identity, profiles.Inspect) {
		return zero, false, nil
	}
	profiles.Apply(&values[match], protocol, identity)
	if err := profiles.Persist(values); err != nil {
		return zero, false, err
	}
	profiles.Commit(values)
	return values[match], true, nil
}

func uniqueSCIMMatch[T any](profiles []T, subject string, inspect func(T) Profile) (int, bool) {
	match := -1
	for index, profile := range profiles {
		fields := inspect(profile)
		if !fields.SCIMManaged || fields.Disabled || fields.SCIMDeleted || fields.SCIMExternalID != subject {
			continue
		}
		if match >= 0 {
			return -1, false
		}
		match = index
	}
	return match, true
}

// Link validates ownership and uniqueness before it persists one identity atomically.
func (profiles Profiles[T]) Link(protocol Protocol, id string, identity Identity) error {
	return profiles.link(protocol, id, identity, "", false)
}

// LinkForSession rechecks the initiating session atomically with the profile write.
func (profiles Profiles[T]) LinkForSession(protocol Protocol, id string, identity Identity, session string) error {
	return profiles.link(protocol, id, identity, session, true)
}

func (profiles Profiles[T]) link(protocol Protocol, id string, identity Identity, session string, requireSession bool) error {
	if err := validateIdentity(protocol, identity); err != nil {
		return err
	}
	profiles.Lock()
	defer profiles.Unlock()
	if requireSession && (session == "" || profiles.SessionActive == nil || !profiles.SessionActive(session, id)) {
		return errors.New("linking session is invalid or expired")
	}
	values := profiles.Clone()
	target := -1
	for index, candidate := range values {
		fields := profiles.Inspect(candidate)
		if fields.ID != id && profileIdentity(fields, protocol) == identity {
			return errors.New(string(protocol) + " identity is already linked")
		}
		if fields.ID == id {
			if fields.Disabled || fields.SCIMDeleted {
				return errors.New("viewer profile is disabled")
			}
			target = index
			if fields.SCIMManaged {
				return errors.New("SCIM-managed profiles are controlled by SCIM provisioning")
			}
		}
	}
	if target < 0 {
		return errors.New("viewer profile was not found")
	}
	profiles.Apply(&values[target], protocol, identity)
	return profiles.persist(values)
}

// Unlink clears one identity and persists the change atomically.
func (profiles Profiles[T]) Unlink(protocol Protocol, id string) error {
	if err := validateProtocol(protocol); err != nil {
		return err
	}
	profiles.Lock()
	defer profiles.Unlock()
	values := profiles.Clone()
	for index, candidate := range values {
		if profiles.Inspect(candidate).ID == id {
			profiles.Apply(&values[index], protocol, Identity{})
			return profiles.persist(values)
		}
	}
	return errors.New("viewer profile was not found")
}

func (profiles Profiles[T]) persist(values []T) error {
	if err := profiles.Persist(values); err != nil {
		return err
	}
	profiles.Commit(values)
	return nil
}

func profileIdentity(profile Profile, protocol Protocol) Identity {
	if protocol == SAMLProtocol {
		return profile.SAML
	}
	return profile.OIDC
}

func identityLinkedElsewhere[T any](profiles []T, match int, protocol Protocol, identity Identity, inspect func(T) Profile) bool {
	for index, profile := range profiles {
		if index != match && profileIdentity(inspect(profile), protocol) == identity {
			return true
		}
	}
	return false
}

func validateIdentity(protocol Protocol, identity Identity) error {
	if err := validateProtocol(protocol); err != nil {
		return err
	}
	if identity.Issuer == "" || !ValidSubject(identity.Subject) {
		return errors.New(string(protocol) + " identity is invalid")
	}
	return nil
}

func validateProtocol(protocol Protocol) error {
	if protocol != OIDCProtocol && protocol != SAMLProtocol {
		return errors.New("federation protocol is invalid")
	}
	return nil
}
