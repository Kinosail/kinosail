package configurationcore

import (
	"fmt"
	"sync"
	"time"
)

const (
	// SCIMTokenKey is the atomic SCIM token setting.
	SCIMTokenKey = "integrations.scim.token"
	// SCIMExpirationKey is the atomic SCIM token expiry setting.
	SCIMExpirationKey = "integrations.scim.token_expires_at"
)

// Mutation adapts app storage and validation to shared configuration changes.
type Mutation struct {
	Lock           sync.Locker
	Find           func(string) (bool, bool)
	CoupledError   func(string) error
	Validate       func(string, string) error
	Read           func(string) (map[string]string, map[string]string, error)
	ValidateStored func(map[string]string, map[string]string) error
	Persist        func(string, map[string]string, map[string]string, bool, bool) error
}

// Set validates and persists one setting.
func (mutation Mutation) Set(dataDir, key, raw string) error {
	secret, ok := mutation.Find(key)
	if !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	if err := mutation.CoupledError(key); err != nil {
		return err
	}
	if err := mutation.Validate(key, raw); err != nil {
		return err
	}
	return mutation.change(dataDir, key, raw, false, secret)
}

// Delete validates and removes one setting.
func (mutation Mutation) Delete(dataDir, key string) error {
	secret, ok := mutation.Find(key)
	if !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	if err := mutation.CoupledError(key); err != nil {
		return err
	}
	return mutation.change(dataDir, key, "", true, secret)
}

// SetSCIM validates and persists the SCIM token and expiry atomically.
func (mutation Mutation) SetSCIM(dataDir, token, expiresAt string) error {
	if token == "" || expiresAt == "" {
		return fmt.Errorf("%s and %s must be configured together", SCIMTokenKey, SCIMExpirationKey)
	}
	if err := mutation.Validate(SCIMTokenKey, token); err != nil {
		return err
	}
	if err := mutation.Validate(SCIMExpirationKey, expiresAt); err != nil {
		return err
	}
	expires, _ := time.Parse(time.RFC3339, expiresAt)
	now := time.Now().UTC()
	if !expires.After(now) || expires.After(now.Add(90*24*time.Hour)) {
		return fmt.Errorf("invalid %s: must be a future RFC3339 time no more than 90 days away", SCIMExpirationKey)
	}
	return mutation.changeSCIM(dataDir, token, expiresAt, false)
}

// DeleteSCIM removes the SCIM token and expiry atomically.
func (mutation Mutation) DeleteSCIM(dataDir string) error {
	return mutation.changeSCIM(dataDir, "", "", true)
}

func (mutation Mutation) change(dataDir, key, raw string, deleted, secret bool) error {
	mutation.Lock.Lock()
	defer mutation.Lock.Unlock()
	regular, secrets, err := mutation.Read(dataDir)
	if err != nil {
		return err
	}
	target := regular
	if secret {
		target = secrets
	}
	if deleted {
		delete(target, key)
	} else {
		target[key] = raw
	}
	if err := mutation.ValidateStored(regular, secrets); err != nil {
		return err
	}
	return mutation.Persist(dataDir, regular, secrets, !secret, secret)
}

func (mutation Mutation) changeSCIM(dataDir, token, expiresAt string, deleted bool) error {
	mutation.Lock.Lock()
	defer mutation.Lock.Unlock()
	regular, secrets, err := mutation.Read(dataDir)
	if err != nil {
		return err
	}
	if deleted {
		delete(secrets, SCIMTokenKey)
		delete(regular, SCIMExpirationKey)
	} else {
		secrets[SCIMTokenKey], regular[SCIMExpirationKey] = token, expiresAt
	}
	if err := mutation.ValidateStored(regular, secrets); err != nil {
		return err
	}
	return mutation.Persist(dataDir, regular, secrets, true, true)
}
