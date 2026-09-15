package configuration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/configuration"
)

func TestSCIMTokenIsValidatedAndRedacted(t *testing.T) { //nolint:cyclop // The table covers configuration precedence and all SCIM token boundaries.
	t.Parallel()
	valid := strings.Repeat("s", 32)
	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	loaded, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		if key == "KINOSAIL_SCIM_TOKEN" {
			return valid, true
		}
		return expires, key == "KINOSAIL_SCIM_TOKEN_EXPIRES_AT"
	})
	if err != nil || loaded.String("integrations.scim.token") != valid || loaded.Public("integrations.scim.token").Value != "" || !loaded.Public("integrations.scim.token").Configured || !loaded.Public("integrations.scim.token").Secret {
		t.Fatalf("SCIM token = %#v, %v", loaded.Public("integrations.scim.token"), err)
	}
	for _, token := range []string{"short", strings.Repeat("s", 257), valid + "\n"} {
		if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
			if key == "KINOSAIL_SCIM_TOKEN" {
				return token, true
			}
			return expires, key == "KINOSAIL_SCIM_TOKEN_EXPIRES_AT"
		}); err == nil {
			t.Fatalf("invalid SCIM token %q was accepted", token)
		}
	}
	if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) { return valid, key == "KINOSAIL_SCIM_TOKEN" }); err == nil {
		t.Fatal("SCIM token without expiration was accepted")
	}
	expired, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		if key == "KINOSAIL_SCIM_TOKEN" {
			return valid, true
		}
		return time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), key == "KINOSAIL_SCIM_TOKEN_EXPIRES_AT"
	})
	if err != nil || expired.String("integrations.scim.token") != valid {
		t.Fatalf("expired SCIM configuration should load with provisioning disabled: %v", err)
	}
}

func TestSCIMConfigurationMutationsValidateTheCompletePair(t *testing.T) { //nolint:cyclop,gocognit // The test keeps paired persistence and no-side-effect boundaries together.
	t.Parallel()
	directory := t.TempDir()
	token := strings.Repeat("s", 32)
	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := configuration.SetSCIM(directory, "", ""); err == nil {
		t.Fatal("empty SCIM pair was accepted")
	}
	if err := configuration.SetSCIM(directory, "short", expires); err == nil {
		t.Fatal("invalid SCIM token pair was accepted")
	}
	for _, invalidExpiration := range []string{time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), time.Now().UTC().Add(91 * 24 * time.Hour).Format(time.RFC3339)} {
		if err := configuration.SetSCIM(directory, token, invalidExpiration); err == nil {
			t.Fatalf("unsafe SCIM expiration %q was accepted", invalidExpiration)
		}
	}
	if err := configuration.Set(directory, "integrations.scim.token", token); err == nil {
		t.Fatal("SCIM token accepted a non-atomic mutation")
	}
	if err := configuration.Set(directory, "integrations.scim.token_expires_at", expires); err == nil {
		t.Fatal("SCIM expiration accepted a non-atomic mutation")
	}
	if _, err := os.Stat(filepath.Join(directory, "secrets.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid SCIM mutation left secrets file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "configuration.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid SCIM mutation left configuration file: %v", err)
	}
	if err := configuration.SetSCIM(directory, token, expires); err != nil {
		t.Fatalf("valid SCIM pair update = %v", err)
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.scim.token") != token || loaded.String("integrations.scim.token_expires_at") != expires {
		t.Fatalf("saved SCIM pair = %q %q err=%v", loaded.String("integrations.scim.token"), loaded.String("integrations.scim.token_expires_at"), err)
	}
	if err := configuration.SetSCIM(directory, strings.Repeat("x", 32), "not-a-time"); err == nil {
		t.Fatal("invalid SCIM pair update was accepted")
	}
	if unchanged, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false }); loadErr != nil || unchanged.String("integrations.scim.token") != token || unchanged.String("integrations.scim.token_expires_at") != expires {
		t.Fatalf("invalid SCIM pair update changed state: %q %q err=%v", unchanged.String("integrations.scim.token"), unchanged.String("integrations.scim.token_expires_at"), loadErr)
	}
	if err := configuration.Delete(directory, "integrations.scim.token"); err == nil {
		t.Fatal("SCIM token accepted a non-atomic deletion")
	}
	if err := configuration.DeleteSCIM(directory); err != nil {
		t.Fatalf("SCIM pair deletion = %v", err)
	}
	if cleared, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false }); loadErr != nil || cleared.String("integrations.scim.token") != "" || cleared.String("integrations.scim.token_expires_at") != "" {
		t.Fatalf("cleared SCIM pair = %q %q err=%v", cleared.String("integrations.scim.token"), cleared.String("integrations.scim.token_expires_at"), loadErr)
	}
}

func TestSCIMConfigurationLoadsFromYAML(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	secretPath := filepath.Join(directory, "scim-token")
	token := strings.Repeat("y", 32)
	if err := os.WriteFile(secretPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	yamlPath := filepath.Join(directory, "kinosail.yaml")
	yaml := "version: 1\nintegrations:\n  scim:\n    token_file: scim-token\n    token_expires_at: \"" + expires + "\"\n"
	if err := os.WriteFile(yamlPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(directory, yamlPath, func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.scim.token") != token || loaded.String("integrations.scim.token_expires_at") != expires || loaded.Source("integrations.scim.token") != configuration.YAML {
		t.Fatalf("YAML SCIM configuration = %q %q source=%q err=%v", loaded.String("integrations.scim.token"), loaded.String("integrations.scim.token_expires_at"), loaded.Source("integrations.scim.token"), err)
	}
}

func TestConfigurationMutationsSerializeAcrossThePair(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	start := make(chan struct{})
	errors := make(chan error, 12)
	tokens := []string{"a", "b", "c", "d", "e", "f"}
	retentions := []string{"1", "2", "3", "4", "5", "6"}
	for index := range 6 {
		go func(index int) {
			<-start
			token := strings.Repeat(tokens[index], 32)
			errors <- configuration.SetSCIM(directory, token, time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
		}(index)
		go func(index int) {
			<-start
			errors <- configuration.Set(directory, "backup.retention", retentions[index])
		}(index)
	}
	close(start)
	for index := 0; index < cap(errors); index++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.scim.token") == "" || loaded.String("integrations.scim.token_expires_at") == "" || loaded.Int("backup.retention") < 1 {
		t.Fatalf("serialized configuration state = %q %q %d err=%v", loaded.String("integrations.scim.token"), loaded.String("integrations.scim.token_expires_at"), loaded.Int("backup.retention"), err)
	}
	if temporary, globErr := filepath.Glob(filepath.Join(directory, ".kinosail-configuration-*")); globErr != nil || len(temporary) != 0 {
		t.Fatalf("temporary configuration files = %#v err=%v", temporary, globErr)
	}
}
