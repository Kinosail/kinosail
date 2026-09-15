package configurationtest

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (configuration contract[Source, PublicValue, Snapshot]) testLoadPrecedenceSourcesAndSecretFiles(t *testing.T) { //nolint:cyclop // One precedence contract deliberately covers every source; exact policy remains below 22.
	t.Parallel()
	directory := t.TempDir()
	WriteAt(t, filepath.Join(directory, "configuration.json"), `{"server.name":"GUI","playback.autoplay":"true"}`)
	yamlPath := filepath.Join(directory, "kinosail.yaml")
	if err := os.WriteFile(yamlPath, []byte("version: 1\nserver:\n  name: YAML\nplayback:\n  mode: direct\nlibraries:\n  - Movies\n  - Shows\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(directory, "tmdb")
	WriteAt(t, secret, "secret-token\n")
	environment := map[string]string{"KINOSAIL_SERVER_NAME": "Environment", "KINOSAIL_REQUIRE_MFA": "true", "KINOSAIL_JELLYFIN_ENABLED": "false", "KINOSAIL_HOME_ASSISTANT_ENABLED": "true", "KINOSAIL_TMDB_TOKEN_FILE": secret}
	loaded, err := configuration.Load(directory, yamlPath, func(name string) (string, bool) { value, ok := environment[name]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	configuration.assertEnvironmentPrecedence(t, loaded)
	if value, source := loaded.String("playback.mode"), loaded.Source("playback.mode"); value != "direct" || source != configuration.YAML {
		t.Fatalf("playback.mode = %q from %q", value, source)
	}
	if value, source := loaded.Bool("playback.autoplay"), loaded.Source("playback.autoplay"); !value || source != configuration.GUI {
		t.Fatalf("playback.autoplay = %v from %q", value, source)
	}
	if libraries := loaded.Strings("libraries"); len(libraries) != 2 || libraries[0] != "Movies" || libraries[1] != "Shows" {
		t.Fatalf("libraries = %#v", libraries)
	}
	if loaded.String("integrations.tmdb.token") != "secret-token" || publicProjection[Source](loaded.Public("integrations.tmdb.token")).Value != "" || !publicProjection[Source](loaded.Public("integrations.tmdb.token")).Configured {
		t.Fatal("secret was not loaded and redacted")
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testLoadRejectsUnknownAndInvalidValues(t *testing.T) {
	t.Parallel()
	for name, contents := range map[string]string{
		"unknown":                "version: 1\nserver:\n  typo: value\n",
		"version":                "version: 2\n",
		"boolean":                "version: 1\nplayback:\n  autoplay: perhaps\n",
		"MFA boolean":            "version: 1\nsecurity:\n  require_mfa: sometimes\n",
		"Jellyfin boolean":       "version: 1\nintegrations:\n  jellyfin:\n    enabled: sometimes\n",
		"Home Assistant boolean": "version: 1\nintegrations:\n  home_assistant:\n    enabled: sometimes\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "kinosail.yaml")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := configuration.Load(t.TempDir(), path, func(string) (string, bool) { return "", false }); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testTLSHostsAreStrictlyBoundedHostnamesAndAddresses(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		value string
		valid bool
	}{
		"hostnames and addresses": {`["media.example","192.0.2.55","2001:db8::1"]`, true},
		"empty list":              {`[]`, true},
		"empty host":              {`[""]`, false},
		"port":                    {"[\"media.example:" + configuration.Port + "\"]", false},
		"wildcard":                {`["*.nox"]`, false},
		"oversized host":          {`["` + strings.Repeat("a", 254) + `"]`, false},
		"too many hosts":          {`[` + strings.Repeat(`"a",`, 32) + `"a"]`, false},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
				return test.value, key == "KINOSAIL_TLS_HOSTS"
			})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
		})
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testTrustedHTTPSConfigurationIsSecretAndCanonical(t *testing.T) { //nolint:cyclop // One lifecycle proves canonicalization, public redaction, and secret persistence.
	t.Parallel()
	directory := t.TempDir()
	raw := `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
	environment := map[string]string{"KINOSAIL_DUCKDNS_HTTPS": raw, "KINOSAIL_LISTEN": ":" + configuration.Port + ""}
	loaded, err := configuration.Load(directory, "", func(key string) (string, bool) { value, ok := environment[key]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	if loaded.String("tls.duckdns") != raw || publicProjection[Source](loaded.Public("tls.duckdns")).Value != "" || !publicProjection[Source](loaded.Public("tls.duckdns")).Configured || loaded.Source("tls.duckdns") != configuration.Environment {
		t.Fatalf("trusted HTTPS public value = %#v", loaded.Public("tls.duckdns"))
	}
	origin, err := configuration.TrustedOrigin(raw, loaded.String("listen"))
	if err != nil || origin != "https://family-media.duckdns.org:"+configuration.Port+"" {
		t.Fatalf("trusted origin = %q, %v", origin, err)
	}
	if err = configuration.Set(directory, "tls.duckdns", raw); err != nil {
		t.Fatal(err)
	}
	regular, _ := os.ReadFile(filepath.Join(directory, "configuration.json"))
	secrets, _ := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if strings.Contains(string(regular), "family-media") || !strings.Contains(string(secrets), "family-media") || !strings.Contains(string(secrets), strings.Repeat("t", 32)) {
		t.Fatalf("trusted HTTPS storage regular=%q secrets=%q", regular, secrets)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testTrustedHTTPSConfigurationRejectsCrossFieldConflicts(t *testing.T) {
	t.Parallel()
	raw := `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
	valid := map[string]string{"KINOSAIL_DUCKDNS_HTTPS": raw, "KINOSAIL_LISTEN": ":" + configuration.Port + ""}
	for name, changes := range map[string]map[string]string{
		"TLS disabled":        {"KINOSAIL_TLS_ENABLED": "false"},
		"wrong auth origin":   {"KINOSAIL_AUTH_URL": "https://other.duckdns.org:" + configuration.Port + ""},
		"nonnumeric port":     {"KINOSAIL_LISTEN": ":https"},
		"public HTTPS mode":   {"KINOSAIL_REMOTE_MODE": "https", "KINOSAIL_DUCKDNS_DOMAIN": "remote-media", "KINOSAIL_DUCKDNS_TOKEN": strings.Repeat("r", 32), "KINOSAIL_AUTH_URL": "https://remote-media.duckdns.org"},
		"shared remote label": {"KINOSAIL_REMOTE_MODE": "wireguard", "KINOSAIL_DUCKDNS_DOMAIN": "family-media", "KINOSAIL_DUCKDNS_TOKEN": strings.Repeat("r", 32)},
	} {
		t.Run(name, func(t *testing.T) {
			environment := make(map[string]string, len(valid)+len(changes))
			maps.Copy(environment, valid)
			maps.Copy(environment, changes)
			if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) { value, ok := environment[key]; return value, ok }); err == nil {
				t.Fatal("conflicting trusted HTTPS configuration was accepted")
			}
		})
	}
	if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		return map[string]string{"KINOSAIL_DUCKDNS_HTTPS": raw, "KINOSAIL_AUTH_URL": "https://family-media.duckdns.org:" + configuration.Port + ""}[key], key == "KINOSAIL_DUCKDNS_HTTPS" || key == "KINOSAIL_AUTH_URL"
	}); err != nil {
		t.Fatalf("matching canonical auth origin: %v", err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testJellyfinCompatibilityRequiresTrustedHTTPSAcrossConfigurationSources(t *testing.T) {
	t.Parallel()
	raw := `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
	if _, err := configuration.Load(t.TempDir(), "", Lookup(map[string]string{"KINOSAIL_JELLYFIN_ENABLED": "true"})); err == nil {
		t.Fatal("Jellyfin compatibility without trusted HTTPS was accepted")
	}
	loaded, err := configuration.Load(t.TempDir(), "", Lookup(map[string]string{"KINOSAIL_JELLYFIN_ENABLED": "true", "KINOSAIL_DUCKDNS_HTTPS": raw}))
	if err != nil || !loaded.Bool("integrations.jellyfin.enabled") {
		t.Fatalf("Jellyfin compatibility with trusted HTTPS = %v, %v", loaded.Bool("integrations.jellyfin.enabled"), err)
	}

	directory := t.TempDir()
	if err = configuration.Set(directory, "integrations.jellyfin.enabled", "true"); err == nil {
		t.Fatal("stored Jellyfin compatibility without trusted HTTPS was accepted")
	}
	regular, readErr := os.ReadFile(filepath.Join(directory, "configuration.json"))
	if readErr == nil && strings.Contains(string(regular), "integrations.jellyfin.enabled") {
		t.Fatalf("rejected Jellyfin configuration changed storage: %q", regular)
	}
	if err = configuration.Set(directory, "tls.duckdns", raw); err != nil {
		t.Fatal(err)
	}
	if err = configuration.Set(directory, "integrations.jellyfin.enabled", "true"); err != nil {
		t.Fatalf("stored Jellyfin compatibility with trusted HTTPS: %v", err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testSecureRemoteAccessConfigurationIsCompleteAndStrict(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"KINOSAIL_REMOTE_MODE":    "https",
		"KINOSAIL_DUCKDNS_DOMAIN": "family-media",
		"KINOSAIL_DUCKDNS_TOKEN":  strings.Repeat("a", 32),
		"KINOSAIL_REMOTE_LISTEN":  ":8443",
		"KINOSAIL_AUTH_URL":       "https://family-media.duckdns.org",
	}
	loaded, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) { value, ok := valid[key]; return value, ok })
	if err != nil || loaded.String("remote.mode") != "https" || publicProjection[Source](loaded.Public("remote.duckdns_token")).Value != "" {
		t.Fatalf("valid secure remote access = %#v, %v", loaded.Public("remote.mode"), err)
	}
	for name, change := range map[string]map[string]string{
		"unknown mode":      {"KINOSAIL_REMOTE_MODE": "proxy"},
		"missing domain":    {"KINOSAIL_DUCKDNS_DOMAIN": ""},
		"full domain":       {"KINOSAIL_DUCKDNS_DOMAIN": "family.duckdns.org"},
		"short token":       {"KINOSAIL_DUCKDNS_TOKEN": "short"},
		"wrong auth origin": {"KINOSAIL_AUTH_URL": "https://other.example"},
		"invalid listen":    {"KINOSAIL_REMOTE_LISTEN": "public"},
	} {
		t.Run(name, func(t *testing.T) {
			environment := make(map[string]string, len(valid))
			maps.Copy(environment, valid)
			maps.Copy(environment, change)
			if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) { value, ok := environment[key]; return value, ok }); err == nil {
				t.Fatal("invalid remote access configuration was accepted")
			}
		})
	}
	wireGuard := map[string]string{"KINOSAIL_REMOTE_MODE": "wireguard", "KINOSAIL_DUCKDNS_DOMAIN": "family-media", "KINOSAIL_DUCKDNS_TOKEN": strings.Repeat("a", 32), "KINOSAIL_WIREGUARD_DIR": "/wireguard"}
	if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) { value, ok := wireGuard[key]; return value, ok }); err != nil {
		t.Fatalf("WireGuard configuration = %v", err)
	}
	wireGuard["KINOSAIL_WIREGUARD_DIR"] = "relative"
	if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) { value, ok := wireGuard[key]; return value, ok }); err == nil {
		t.Fatal("relative WireGuard directory was accepted")
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testSupporterConfigurationRequiresSafeHTTPSURLs(t *testing.T) {
	for _, values := range []map[string]string{
		{"KINOSAIL_SUPPORTER_ACTIVATION_URL": "http://support.example/activate"},
		{"KINOSAIL_SUPPORTER_ACTIVATION_URL": "https://user@support.example/activate"},
		{"KINOSAIL_SUPPORT_URL": "javascript:alert(1)"},
		{"KINOSAIL_SUPPORT_URL": "https://support.example/#account"},
	} {
		if _, err := configuration.Load(t.TempDir(), "", Lookup(values)); err == nil {
			t.Fatalf("unsafe supporter configuration %#v was accepted", values)
		}
	}
	configured, err := configuration.Load(t.TempDir(), "", Lookup(map[string]string{
		"KINOSAIL_SUPPORTER_ACTIVATION_URL": "http://127.0.0.1:41905/v1/activate",
		"KINOSAIL_SUPPORT_URL":              "https://buy.polar.sh/polar_cl_example",
	}))
	if err != nil || configured.String("supporter.activation_url") != "http://127.0.0.1:41905/v1/activate" || configured.String("supporter.url") != "https://buy.polar.sh/polar_cl_example" {
		t.Fatalf("valid supporter configuration = %#v, %v", configured, err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testSetPersistsSecretsSeparatelyAndHonorsManagedFields(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := configuration.Set(directory, "backup.retention", "12"); err != nil {
		t.Fatal(err)
	}
	if err := configuration.Set(directory, "integrations.tmdb.token", "secret-token"); err != nil {
		t.Fatal(err)
	}
	regular, _ := os.ReadFile(filepath.Join(directory, "configuration.json"))
	secrets, _ := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if strings.Contains(string(regular), "secret-token") || !strings.Contains(string(secrets), "secret-token") {
		t.Fatalf("regular=%q secrets=%q", regular, secrets)
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.Int("backup.retention") != 12 || loaded.Source("backup.retention") != configuration.GUI {
		t.Fatalf("loaded=%v source=%q err=%v", loaded.Int("backup.retention"), loaded.Source("backup.retention"), err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testExampleYAMLLoads(t *testing.T) {
	t.Parallel()
	loaded, err := configuration.Load(t.TempDir(), filepath.Join("..", "..", "kinosail.example.yaml"), func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("server.name") != "" || loaded.Source("server.name") != configuration.Default {
		t.Fatalf("example name=%q source=%q err=%v", loaded.String("server.name"), loaded.Source("server.name"), err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testContainerDefaultsRemainGUIEditable(t *testing.T) {
	t.Parallel()
	loaded, err := configuration.Load(t.TempDir(), "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("backup.directory") != "/backups" || loaded.Source("backup.retention") != configuration.Default {
		t.Fatalf("backup=%q retention source=%q err=%v", loaded.String("backup.directory"), loaded.Source("backup.retention"), err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) testYAMLDataDirectoryBootstrapsGUIState(t *testing.T) {
	t.Parallel()
	root, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "configuration.json"), []byte(`{"server.name":"GUI in configured data"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(root, "kinosail.yaml")
	if err := os.WriteFile(yamlPath, []byte("version: 1\npaths:\n  data: "+dataDir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(root, yamlPath, func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("server.name") != "GUI in configured data" || loaded.Source("server.name") != configuration.GUI {
		t.Fatalf("name=%q source=%q err=%v", loaded.String("server.name"), loaded.Source("server.name"), err)
	}
}

func (configuration contract[Source, PublicValue, Snapshot]) assertEnvironmentPrecedence(t *testing.T, loaded Snapshot) {
	t.Helper()
	if value, source := loaded.String("server.name"), loaded.Source("server.name"); value != "Environment" || source != configuration.Environment {
		t.Fatalf("server.name = %q from %q", value, source)
	}
	if !loaded.Bool("security.require_mfa") || loaded.Source("security.require_mfa") != configuration.Environment || loaded.Bool("integrations.jellyfin.enabled") || loaded.Source("integrations.jellyfin.enabled") != configuration.Environment {
		t.Fatalf("operator toggles = MFA %v from %q, Jellyfin %v from %q", loaded.Bool("security.require_mfa"), loaded.Source("security.require_mfa"), loaded.Bool("integrations.jellyfin.enabled"), loaded.Source("integrations.jellyfin.enabled"))
	}
	if !loaded.Bool("integrations.home_assistant.enabled") || loaded.Source("integrations.home_assistant.enabled") != configuration.Environment {
		t.Fatalf("Home Assistant setting = %v from %q", loaded.Bool("integrations.home_assistant.enabled"), loaded.Source("integrations.home_assistant.enabled"))
	}
}
