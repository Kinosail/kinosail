package identitycore

import (
	"crypto/hmac"
	"errors"
	"slices"
	"strings"
	"time"
)

var (
	ErrInvalidFactorInput = errors.New("multi-factor authentication input is invalid")
	ErrOwnerFactorNeeded  = errors.New("owner must retain a passkey or authenticator")
)

type FactorProfile struct {
	ID       string
	Owner    bool
	Passkeys int
	Secret   string
	Recovery []string
	Counter  int64
	Revision uint64
}

type FactorConfig[P any] struct {
	Profiles *[]P
	Sessions *map[string]Session
	Project  func(P) FactorProfile
	Apply    func(P, FactorProfile) P
	Persist  func([]P, map[string]Session, bool) error
	Now      func() time.Time
}

type FactorTransaction[P any] struct{ config FactorConfig[P] }

func NewFactorTransaction[P any](config FactorConfig[P]) *FactorTransaction[P] {
	if config.Now == nil {
		config.Now = time.Now
	}
	return &FactorTransaction[P]{config: config}
}

func (transaction *FactorTransaction[P]) Enable(id, secret string, recovery []string) error {
	if !transaction.valid() || !validFactorIdentity(id) || !validFactorSecret(secret) {
		return ErrInvalidFactorInput
	}
	hashed, valid := recoveryHashes(recovery)
	if !valid {
		return ErrInvalidFactorInput
	}
	profiles := slices.Clone(*transaction.config.Profiles)
	for index, profile := range profiles {
		factor := transaction.config.Project(profile)
		if factor.ID != id {
			continue
		}
		factor.Secret, factor.Recovery, factor.Counter = secret, hashed, 0
		factor.Revision++
		profiles[index] = transaction.config.Apply(profile, factor)
		sessions := WithoutProfile(*transaction.config.Sessions, id, true)
		return transaction.commit(profiles, sessions, true)
	}
	return ErrProfileNotFound
}

func (transaction *FactorTransaction[P]) Disable(id string) error {
	if !transaction.valid() || !validFactorIdentity(id) {
		return ErrProfileNotFound
	}
	profiles := slices.Clone(*transaction.config.Profiles)
	for index, profile := range profiles {
		factor := transaction.config.Project(profile)
		if factor.ID != id {
			continue
		}
		if factor.Owner && factor.Passkeys == 0 {
			return ErrOwnerFactorNeeded
		}
		factor.Secret, factor.Recovery, factor.Counter = "", nil, 0
		factor.Revision++
		profiles[index] = transaction.config.Apply(profile, factor)
		sessions := WithoutProfile(*transaction.config.Sessions, id, true)
		return transaction.commit(profiles, sessions, true)
	}
	return ErrProfileNotFound
}

func (transaction *FactorTransaction[P]) Verify(id, code string) bool { //nolint:cyclop,gocognit // TOTP and one-time recovery verification share one atomic factor boundary.
	if !transaction.valid() || !validFactorIdentity(id) || code == "" || len(code) > maxSecretLength || strings.ContainsRune(code, '\x00') {
		return false
	}
	for index, profile := range *transaction.config.Profiles {
		factor := transaction.config.Project(profile)
		if factor.ID != id || factor.Secret == "" {
			continue
		}
		if counter, valid := TOTPCounter(factor.Secret, code, transaction.config.Now()); valid {
			if counter <= factor.Counter {
				return false
			}
			factor.Counter = counter
			return transaction.commitFactor(index, profile, factor)
		}
		if len(factor.Recovery) > recoveryCodeCount {
			continue
		}
		normalized := NormalizeRecovery(code)
		if normalized == "" {
			return false
		}
		hashed := SessionKey(normalized)
		for position, recovery := range factor.Recovery {
			if !hmac.Equal([]byte(recovery), []byte(hashed)) {
				continue
			}
			factor.Recovery = append(slices.Clone(factor.Recovery[:position]), factor.Recovery[position+1:]...)
			return transaction.commitFactor(index, profile, factor)
		}
	}
	return false
}

func (transaction *FactorTransaction[P]) commitFactor(index int, profile P, factor FactorProfile) bool {
	profiles := slices.Clone(*transaction.config.Profiles)
	profiles[index] = transaction.config.Apply(profile, factor)
	return transaction.commit(profiles, *transaction.config.Sessions, false) == nil
}

func (transaction *FactorTransaction[P]) valid() bool {
	return transaction != nil && transaction.config.Profiles != nil && transaction.config.Sessions != nil && transaction.config.Project != nil && transaction.config.Apply != nil && transaction.config.Persist != nil && transaction.config.Now != nil
}

func (transaction *FactorTransaction[P]) commit(profiles []P, sessions map[string]Session, related bool) error {
	if err := transaction.config.Persist(profiles, sessions, related); err != nil {
		return err
	}
	*transaction.config.Profiles = profiles
	if related {
		*transaction.config.Sessions = sessions
	}
	return nil
}

func validFactorIdentity(id string) bool {
	return id != "" && len(id) <= maxIdentityLength && id == strings.TrimSpace(id) && !strings.ContainsRune(id, '\x00')
}

func validFactorSecret(secret string) bool {
	return secret != "" && len(secret) <= maxSecretLength && secret == strings.TrimSpace(secret) && !strings.ContainsRune(secret, '\x00')
}

func recoveryHashes(recovery []string) ([]string, bool) {
	if len(recovery) == 0 || len(recovery) > recoveryCodeCount {
		return nil, false
	}
	hashed, unique := make([]string, len(recovery)), make(map[string]struct{}, len(recovery))
	for index, code := range recovery {
		normalized := NormalizeRecovery(code)
		if normalized == "" {
			return nil, false
		}
		hashed[index] = SessionKey(normalized)
		if _, found := unique[hashed[index]]; found {
			return nil, false
		}
		unique[hashed[index]] = struct{}{}
	}
	return hashed, true
}
