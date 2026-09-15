// Package identitycore owns shared account security rules for Kinosail apps.
package identitycore

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 requires HMAC-SHA-1.
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	recoveryCodeCount = 10
	maxIdentityLength = 100
	maxNameLength     = 64
	maxSecretLength   = 128
)

var (
	ErrInvalidConfig     = errors.New("invalid identity configuration")
	ErrInvalidInput      = errors.New("invalid identity input")
	ErrInvalidEnrollment = errors.New("authentication code is invalid or enrollment expired")
)

// Enrollment contains one time-based one-time password enrollment.
type Enrollment struct {
	Secret, URI   string
	RecoveryCodes []string
	Expires       time.Time
}

// MFA owns bounded in-flight multi-factor authentication enrollments.
type MFA struct {
	mu      sync.Mutex
	pending map[string]Enrollment
	now     func() time.Time
	random  io.Reader
}

// NewMFA creates a multi-factor authentication engine.
func NewMFA() *MFA {
	return &MFA{pending: make(map[string]Enrollment), now: time.Now, random: rand.Reader}
}

// Setup creates and stores one enrollment after identity validation.
func (mfa *MFA) Setup(identity, name string) (Enrollment, error) {
	if !validIdentity(identity, name) || mfa == nil || mfa.now == nil || mfa.random == nil {
		return Enrollment{}, ErrInvalidInput
	}
	secretBytes := make([]byte, 20)
	if _, err := io.ReadFull(mfa.random, secretBytes); err != nil {
		return Enrollment{}, err
	}
	codes := make([]string, recoveryCodeCount)
	for index := range codes {
		value := make([]byte, 8)
		if _, err := io.ReadFull(mfa.random, value); err != nil {
			return Enrollment{}, err
		}
		raw := strings.ToUpper(hex.EncodeToString(value))
		codes[index] = raw[:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:]
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	issuer, account := url.QueryEscape("Kinosail"), url.QueryEscape(name)
	enrollment := Enrollment{Secret: secret, URI: "otpauth://totp/" + issuer + ":" + account + "?secret=" + secret + "&issuer=" + issuer + "&digits=6&period=30", RecoveryCodes: codes, Expires: mfa.now().Add(10 * time.Minute)}
	mfa.mu.Lock()
	mfa.pending[identity] = enrollment
	mfa.mu.Unlock()
	return cloneEnrollment(enrollment), nil
}

// Confirm verifies and consumes one enrollment.
func (mfa *MFA) Confirm(identity, code string) (Enrollment, error) {
	if mfa == nil || mfa.now == nil || identity == "" || len(identity) > maxIdentityLength || len(code) > 32 {
		return Enrollment{}, ErrInvalidEnrollment
	}
	mfa.mu.Lock()
	defer mfa.mu.Unlock()
	enrollment, found := mfa.pending[identity]
	now := mfa.now()
	if !found || now.After(enrollment.Expires) || !ValidTOTP(enrollment.Secret, code, now) {
		return Enrollment{}, ErrInvalidEnrollment
	}
	delete(mfa.pending, identity)
	return cloneEnrollment(enrollment), nil
}

// Discard removes one pending enrollment.
func (mfa *MFA) Discard(identity string) {
	if mfa == nil || identity == "" || len(identity) > maxIdentityLength {
		return
	}
	mfa.mu.Lock()
	delete(mfa.pending, identity)
	mfa.mu.Unlock()
}

// PendingCount reports the number of in-flight enrollments.
func (mfa *MFA) PendingCount() int {
	if mfa == nil {
		return 0
	}
	mfa.mu.Lock()
	defer mfa.mu.Unlock()
	return len(mfa.pending)
}

// ValidTOTP verifies one RFC 6238 code with one adjacent time step.
func ValidTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(secret) == 0 {
		return false
	}
	if len(secret) > maxSecretLength {
		return false
	}
	if len(code) != 6 {
		return false
	}
	if !decimal(code) {
		return false
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return false
	}
	counter := now.Unix() / 30
	if hmac.Equal([]byte(totp(key, counter)), []byte(code)) {
		return true
	}
	if hmac.Equal([]byte(totp(key, counter-1)), []byte(code)) {
		return true
	}
	return hmac.Equal([]byte(totp(key, counter+1)), []byte(code))
}

// NormalizeRecovery canonicalizes one bounded recovery code.
func NormalizeRecovery(code string) string {
	if len(code) > 128 {
		return ""
	}
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
}

func totp(key []byte, counterValue int64) string {
	if counterValue < 0 {
		return ""
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(counterValue))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	digest := mac.Sum(nil)
	position := digest[len(digest)-1] & 0x0f
	value := binary.BigEndian.Uint32(digest[position:position+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}

func decimal(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validIdentity(identity, name string) bool {
	return identity != "" && len(identity) <= maxIdentityLength && name != "" && len(name) <= maxNameLength
}

func cloneEnrollment(enrollment Enrollment) Enrollment {
	enrollment.RecoveryCodes = append([]string(nil), enrollment.RecoveryCodes...)
	return enrollment
}
