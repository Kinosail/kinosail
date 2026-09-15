package passkeys

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	MaxCredentials  = 20
	maxCredentialID = 1024
	maxPublicKey    = 8192
	maxUserHandle   = 100
	maxTransports   = 10
	maxAttestation  = 64 << 10
)

var (
	ErrInvalidCredential = errors.New("invalid passkey credential")
	ErrDuplicate         = errors.New("passkey is already registered")
	ErrLimit             = errors.New("passkey limit reached")
	ErrInvalidID         = errors.New("invalid passkey id")
	ErrNotFound          = errors.New("passkey was not found")
	ErrLastFactor        = errors.New("owner must retain a passkey or authenticator")
)

// User carries an application identity through a WebAuthn ceremony.
type User[T any] struct {
	Value       T
	ID          string
	Name        string
	Credentials []webauthn.Credential
}

func (user User[T]) WebAuthnID() []byte                         { return []byte(user.ID) }
func (user User[T]) WebAuthnName() string                       { return user.Name }
func (user User[T]) WebAuthnDisplayName() string                { return user.Name }
func (user User[T]) WebAuthnCredentials() []webauthn.Credential { return user.Credentials }

// NewUser returns a WebAuthn user with isolated credential storage.
func NewUser[T any](value T, id, name string, credentials []webauthn.Credential) User[T] {
	return User[T]{Value: value, ID: id, Name: name, Credentials: CloneAll(credentials)}
}

// ValidateCredential rejects credentials that cannot be stored safely.
func ValidateCredential(credential *webauthn.Credential) error {
	if !validCredentialShape(credential) {
		return ErrInvalidCredential
	}
	for _, transport := range credential.Transport {
		if !validTransport(transport) {
			return ErrInvalidCredential
		}
	}
	if !validAttestation(credential.Attestation) {
		return ErrInvalidCredential
	}
	return nil
}

func validCredentialShape(credential *webauthn.Credential) bool {
	return credential != nil && len(credential.ID) > 0 && len(credential.ID) <= maxCredentialID && len(credential.PublicKey) > 0 && len(credential.PublicKey) <= maxPublicKey && len(credential.Transport) <= maxTransports && len(credential.Authenticator.AAGUID) <= 16
}

func validAttestation(attestation webauthn.CredentialAttestation) bool {
	return len(attestation.ClientDataJSON) <= maxAttestation && len(attestation.ClientDataHash) <= 64 && len(attestation.AuthenticatorData) <= maxAttestation && len(attestation.Object) <= maxAttestation
}

// ValidateCredentials rejects invalid, duplicate, or excessive stored credentials.
func ValidateCredentials(credentials []webauthn.Credential) error {
	if len(credentials) > MaxCredentials {
		return ErrLimit
	}
	seen := make(map[string]struct{}, len(credentials))
	for index := range credentials {
		if err := ValidateCredential(&credentials[index]); err != nil {
			return err
		}
		key := string(credentials[index].ID)
		if _, found := seen[key]; found {
			return ErrDuplicate
		}
		seen[key] = struct{}{}
	}
	return nil
}

// ValidateLookup bounds a discoverable credential lookup before storage access.
func ValidateLookup(rawID, userHandle []byte) error {
	if len(rawID) == 0 || len(rawID) > maxCredentialID || len(userHandle) == 0 || len(userHandle) > maxUserHandle {
		return ErrInvalidCredential
	}
	return nil
}

// Index finds a credential by its raw identifier.
func Index(credentials []webauthn.Credential, rawID []byte) int {
	for index := range credentials {
		if bytes.Equal(credentials[index].ID, rawID) {
			return index
		}
	}
	return -1
}

// Clone returns a credential without shared byte slices.
func Clone(credential webauthn.Credential) webauthn.Credential {
	credential.ID, credential.PublicKey = slices.Clone(credential.ID), slices.Clone(credential.PublicKey)
	credential.Transport, credential.Authenticator.AAGUID = slices.Clone(credential.Transport), slices.Clone(credential.Authenticator.AAGUID)
	credential.Attestation.ClientDataJSON, credential.Attestation.ClientDataHash = slices.Clone(credential.Attestation.ClientDataJSON), slices.Clone(credential.Attestation.ClientDataHash)
	credential.Attestation.AuthenticatorData, credential.Attestation.Object = slices.Clone(credential.Attestation.AuthenticatorData), slices.Clone(credential.Attestation.Object)
	return credential
}

// CloneAll returns credentials without shared byte slices.
func CloneAll(credentials []webauthn.Credential) []webauthn.Credential {
	cloned := make([]webauthn.Credential, len(credentials))
	for index := range credentials {
		cloned[index] = Clone(credentials[index])
	}
	return cloned
}

// ID returns the redacted storage identifier for a credential.
func ID(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// ValidateID accepts only a lowercase SHA-256 identifier.
func ValidateID(id string) error {
	if len(id) != sha256.Size*2 {
		return ErrInvalidID
	}
	for _, character := range id {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return ErrInvalidID
		}
	}
	return nil
}

// Usage records safe passkey activity metadata.
type Usage struct {
	LastUsed int64 `json:"lastUsed,omitempty"`
	Tracked  bool  `json:"tracked,omitempty"`
}

// Summary is the redacted passkey inventory shape.
type Summary struct {
	ID             string `json:"id"`
	Display        string `json:"display"`
	SignCount      uint32 `json:"signCount"`
	LastUsed       int64  `json:"lastUsed,omitempty"`
	UsageTracked   bool   `json:"usageTracked"`
	LastUsedLabel  string `json:"-"`
	MostRecent     bool   `json:"-"`
	BackupEligible bool   `json:"backupEligible"`
	BackedUp       bool   `json:"backedUp"`
	CloneWarning   bool   `json:"cloneWarning"`
}

// Inventory returns redacted passkey data for account views.
func Inventory(credentials []webauthn.Credential, usage map[string]Usage) []Summary {
	result := make([]Summary, 0, len(credentials))
	mostRecent := int64(0)
	for _, item := range usage {
		if item.LastUsed > mostRecent {
			mostRecent = item.LastUsed
		}
	}
	for _, credential := range credentials {
		id := ID(credential.ID)
		activity := usage[id]
		item := Summary{ID: id, Display: id[:12], SignCount: credential.Authenticator.SignCount, LastUsed: activity.LastUsed, UsageTracked: activity.Tracked, BackupEligible: credential.Flags.BackupEligible, BackedUp: credential.Flags.BackupState, CloneWarning: credential.Authenticator.CloneWarning}
		if activity.LastUsed != 0 {
			item.LastUsedLabel = time.Unix(activity.LastUsed, 0).Local().Format("Jan 2, 2006 at 3:04 PM")
			item.MostRecent = activity.LastUsed == mostRecent
		}
		result = append(result, item)
	}
	return result
}

// Update replaces one verified credential and records its use.
func Update(credentials []webauthn.Credential, usage map[string]Usage, credential *webauthn.Credential, now time.Time) bool {
	if usage == nil || ValidateCredential(credential) != nil {
		return false
	}
	index := Index(credentials, credential.ID)
	if index < 0 {
		return false
	}
	credentials[index] = Clone(*credential)
	id := ID(credential.ID)
	activity := usage[id]
	activity.Tracked = true
	if timestamp := now.Unix(); timestamp > activity.LastUsed {
		activity.LastUsed = timestamp
	}
	usage[id] = activity
	return true
}

func validTransport(transport protocol.AuthenticatorTransport) bool {
	switch transport {
	case protocol.USB, protocol.NFC, protocol.BLE, protocol.SmartCard, protocol.Hybrid, protocol.Internal:
		return true
	default:
		return false
	}
}
