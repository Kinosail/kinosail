package federationconfig

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

type oidcConfigurationEffects struct {
	current, managedKey, source, directory string
	config                                 OIDCConfig
	setErr, deleteErr                      error
	sets, deletes                          int
	updates                                map[string]string
	deleted                                map[string]bool
}

func TestOIDCConfigurationKeysAndView(t *testing.T) {
	t.Parallel()
	keys := OIDCConfigurationKeys()
	keys[0] = "changed"
	if OIDCConfigurationKeys()[0] != OIDCIssuerKey {
		t.Fatal("configuration keys exposed shared state")
	}
	values := map[string]struct {
		value      string
		configured bool
	}{
		OIDCIssuerKey:   {"https://identity.example", true},
		OIDCClientIDKey: {"client", true}, OIDCSecretKey: {"secret", true},
		OIDCRedirectKey: {"https://media.example/login/oidc/callback", true}, OIDCIdentityKey: {"oid", true},
	}
	view := NewOIDCConfigurationView(func(key string) (string, bool) { value := values[key]; return value.value, value.configured }, "control")
	if view.Issuer != values[OIDCIssuerKey].value || view.ClientID != "client" || view.RedirectURL != values[OIDCRedirectKey].value || view.IdentityClaim != "oid" || !view.Configured || !view.SecretConfigured || view.Control != "control" {
		t.Fatalf("view = %#v", view)
	}
}

func TestOIDCConfigurationStoreRejectsBeforeSideEffects(t *testing.T) {
	t.Parallel()
	effects := &oidcConfigurationEffects{}
	store := testOIDCConfigurationStore(effects)
	store.File = ""
	if err := store.Change(OIDCConfig{}, false); err == nil || effects.sets != 0 {
		t.Fatalf("missing storage = %v %#v", err, effects)
	}
	store.File, effects.managedKey, effects.source = "/data/settings.json", OIDCIssuerKey, "yaml"
	if err := store.Change(OIDCConfig{}, false); err == nil || !strings.Contains(err.Error(), "yaml") || effects.sets != 0 {
		t.Fatalf("managed storage = %v %#v", err, effects)
	}
	effects.managedKey, effects.setErr = "", errors.New("invalid settings")
	if err := store.Change(OIDCConfig{}, false); !errors.Is(err, effects.setErr) || effects.sets != 1 || len(effects.updates) != 0 {
		t.Fatalf("failed set = %v %#v", err, effects)
	}
}

func TestOIDCConfigurationStoreSetAndReset(t *testing.T) { //nolint:cyclop // One lifecycle verifies retained, explicit, reset, and failed values.
	t.Parallel()
	effects := &oidcConfigurationEffects{current: "current-secret"}
	store := testOIDCConfigurationStore(effects)
	config := OIDCConfig{Issuer: "issuer", ClientID: "client", RedirectURL: "redirect", IdentityClaim: "sub"}
	if err := store.Change(config, false); err != nil {
		t.Fatal(err)
	}
	if effects.directory != "/data" || effects.config.ClientSecret != "current-secret" || effects.updates[OIDCSecretKey] != "current-secret" || len(effects.updates) != 5 {
		t.Fatalf("set effects = %#v", effects)
	}
	effects.updates, effects.deleted = map[string]string{}, map[string]bool{}
	config.ClientSecret = "explicit"
	if err := store.Change(config, false); err != nil || effects.config.ClientSecret != "explicit" {
		t.Fatalf("explicit set = %v %#v", err, effects)
	}
	if err := store.Change(OIDCConfig{}, true); err != nil || effects.deletes != 1 || len(effects.deleted) != 5 {
		t.Fatalf("reset effects = %v %#v", err, effects)
	}
	effects.deleteErr = errors.New("delete failed")
	if err := store.Change(OIDCConfig{}, true); !errors.Is(err, effects.deleteErr) || effects.deletes != 2 {
		t.Fatalf("failed reset = %v %#v", err, effects)
	}
}

func testOIDCConfigurationStore(effects *oidcConfigurationEffects) OIDCConfigurationStore[string] {
	effects.updates, effects.deleted = map[string]string{}, map[string]bool{}
	return NewOIDCConfigurationStore(
		"/data/settings.json", &sync.Mutex{}, func(string) string { return effects.current },
		func(key string) bool { return key == effects.managedKey }, func(string) string { return effects.source },
		func(directory, issuer, clientID, secret, redirectURL, identityClaim string) error {
			effects.sets++
			effects.directory = directory
			effects.config = OIDCConfig{Issuer: issuer, ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, IdentityClaim: identityClaim}
			return effects.setErr
		},
		func(directory string) error {
			effects.deletes++
			effects.directory = directory
			return effects.deleteErr
		},
		func(key, value string, deleted bool) {
			effects.updates[key], effects.deleted[key] = value, deleted
		},
	)
}
