package configurationcore

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type mutationFixture struct {
	readErr, storedErr, persistErr, coupledErr error
	validateKey                                string
	persists                                   int
	regular, secrets                           map[string]string
	writeRegular, writeSecrets                 bool
}

func (fixture *mutationFixture) mutation() Mutation {
	return Mutation{
		Lock: &sync.Mutex{},
		Find: func(key string) (bool, bool) {
			switch key {
			case "public", SCIMExpirationKey:
				return false, true
			case "secret", SCIMTokenKey:
				return true, true
			default:
				return false, false
			}
		},
		CoupledError: func(string) error { return fixture.coupledErr },
		Validate: func(key, _ string) error {
			if key == fixture.validateKey {
				return errors.New("validation failed")
			}
			return nil
		},
		Read: func(string) (map[string]string, map[string]string, error) {
			if fixture.readErr != nil {
				return nil, nil, fixture.readErr
			}
			return map[string]string{"public": "old", SCIMExpirationKey: "old"}, map[string]string{"secret": "old", SCIMTokenKey: "old"}, nil
		},
		ValidateStored: func(regular, secrets map[string]string) error {
			fixture.regular, fixture.secrets = regular, secrets
			return fixture.storedErr
		},
		Persist: func(_ string, regular, secrets map[string]string, writeRegular, writeSecrets bool) error {
			fixture.persists++
			fixture.regular, fixture.secrets = regular, secrets
			fixture.writeRegular, fixture.writeSecrets = writeRegular, writeSecrets
			return fixture.persistErr
		},
	}
}

func TestMutationRejectsBeforePersistence(t *testing.T) { //nolint:funlen // One table verifies all validation failures precede persistence.
	t.Parallel()
	validation := errors.New("failed")
	tests := map[string]func(*mutationFixture) error{
		"unknown set": func(f *mutationFixture) error { return f.mutation().Set("data", "unknown", "value") },
		"coupled set": func(f *mutationFixture) error {
			f.coupledErr = validation
			return f.mutation().Set("data", "public", "value")
		},
		"invalid set": func(f *mutationFixture) error {
			f.validateKey = "public"
			return f.mutation().Set("data", "public", "value")
		},
		"unknown delete": func(f *mutationFixture) error { return f.mutation().Delete("data", "unknown") },
		"coupled delete": func(f *mutationFixture) error {
			f.coupledErr = validation
			return f.mutation().Delete("data", "public")
		},
		"read set": func(f *mutationFixture) error {
			f.readErr = validation
			return f.mutation().Set("data", "public", "value")
		},
		"stored set": func(f *mutationFixture) error {
			f.storedErr = validation
			return f.mutation().Set("data", "public", "value")
		},
		"missing SCIM": func(f *mutationFixture) error { return f.mutation().SetSCIM("data", "", "") },
		"invalid token": func(f *mutationFixture) error {
			f.validateKey = SCIMTokenKey
			return f.mutation().SetSCIM("data", "token", time.Now().Add(time.Hour).Format(time.RFC3339))
		},
		"invalid expiration": func(f *mutationFixture) error {
			f.validateKey = SCIMExpirationKey
			return f.mutation().SetSCIM("data", "token", "invalid")
		},
		"expired SCIM": func(f *mutationFixture) error {
			return f.mutation().SetSCIM("data", "token", time.Now().Add(-time.Hour).Format(time.RFC3339))
		},
		"distant SCIM": func(f *mutationFixture) error {
			return f.mutation().SetSCIM("data", "token", time.Now().Add(91*24*time.Hour).Format(time.RFC3339))
		},
		"read SCIM": func(f *mutationFixture) error {
			f.readErr = validation
			return f.mutation().SetSCIM("data", "token", time.Now().Add(time.Hour).Format(time.RFC3339))
		},
		"stored SCIM": func(f *mutationFixture) error {
			f.storedErr = validation
			return f.mutation().SetSCIM("data", "token", time.Now().Add(time.Hour).Format(time.RFC3339))
		},
		"read SCIM delete": func(f *mutationFixture) error { f.readErr = validation; return f.mutation().DeleteSCIM("data") },
	}
	for name, operation := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := &mutationFixture{}
			if err := operation(fixture); err == nil {
				t.Fatal("invalid mutation succeeded")
			}
			if fixture.persists != 0 {
				t.Fatal("rejected mutation persisted state")
			}
		})
	}
}

func TestMutationPersistsPublicSecretAndSCIMChanges(t *testing.T) { //nolint:cyclop,funlen,gocognit // One transaction matrix verifies public, secret, and SCIM persistence.
	t.Parallel()
	for name, test := range map[string]struct {
		operation                  func(Mutation) error
		writeRegular, writeSecrets bool
	}{
		"set public":    {func(m Mutation) error { return m.Set("data", "public", "new") }, true, false},
		"set secret":    {func(m Mutation) error { return m.Set("data", "secret", "new") }, false, true},
		"delete public": {func(m Mutation) error { return m.Delete("data", "public") }, true, false},
		"delete secret": {func(m Mutation) error { return m.Delete("data", "secret") }, false, true},
		"set SCIM": {func(m Mutation) error {
			return m.SetSCIM("data", "new-token", time.Now().Add(time.Hour).Format(time.RFC3339))
		}, true, true},
		"delete SCIM": {func(m Mutation) error { return m.DeleteSCIM("data") }, true, true},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := &mutationFixture{}
			if err := test.operation(fixture.mutation()); err != nil {
				t.Fatal(err)
			}
			if fixture.persists != 1 || fixture.writeRegular != test.writeRegular || fixture.writeSecrets != test.writeSecrets {
				t.Fatalf("persist = %d, %t, %t", fixture.persists, fixture.writeRegular, fixture.writeSecrets)
			}
			switch name {
			case "set public":
				if fixture.regular["public"] != "new" {
					t.Fatal("public value was not set")
				}
			case "set secret":
				if fixture.secrets["secret"] != "new" {
					t.Fatal("secret value was not set")
				}
			case "delete public":
				if _, ok := fixture.regular["public"]; ok {
					t.Fatal("public value was not deleted")
				}
			case "delete secret":
				if _, ok := fixture.secrets["secret"]; ok {
					t.Fatal("secret value was not deleted")
				}
			case "set SCIM":
				if fixture.secrets[SCIMTokenKey] != "new-token" || fixture.regular[SCIMExpirationKey] == "old" {
					t.Fatal("SCIM values were not set")
				}
			case "delete SCIM":
				if _, token := fixture.secrets[SCIMTokenKey]; token {
					t.Fatal("SCIM token was not deleted")
				}
				if _, expiration := fixture.regular[SCIMExpirationKey]; expiration {
					t.Fatal("SCIM expiration was not deleted")
				}
			}
		})
	}
	for name, operation := range map[string]func(Mutation) error{
		"ordinary": func(m Mutation) error { return m.Set("data", "public", "new") },
		"SCIM": func(m Mutation) error {
			return m.SetSCIM("data", "new-token", time.Now().Add(time.Hour).Format(time.RFC3339))
		},
	} {
		t.Run(name+" persist error", func(t *testing.T) {
			fixture := &mutationFixture{persistErr: errors.New("persist failed")}
			if err := operation(fixture.mutation()); err == nil || fixture.persists != 1 {
				t.Fatalf("persist failure = %v, calls = %d", err, fixture.persists)
			}
		})
	}
}
