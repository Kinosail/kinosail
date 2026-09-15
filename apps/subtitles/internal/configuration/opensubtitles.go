package configuration

import (
	"errors"
	"fmt"
	"slices"
)

const (
	openSubtitlesAPIKey   = "integrations.opensubtitles.api_key"
	openSubtitlesUsername = "integrations.opensubtitles.username"
	openSubtitlesPassword = "integrations.opensubtitles.password"
)

var openSubtitlesKeys = []string{openSubtitlesAPIKey, openSubtitlesUsername, openSubtitlesPassword}

func isOpenSubtitlesKey(key string) bool {
	return slices.Contains(openSubtitlesKeys, key)
}

func validateOpenSubtitles(configured Snapshot) error {
	present := 0
	for _, key := range openSubtitlesKeys {
		if configured.String(key) != "" {
			present++
		}
	}
	if present != 0 && present != len(openSubtitlesKeys) {
		return errors.New("OpenSubtitles requires API key, username, and password together")
	}
	return nil
}

// SetOpenSubtitles saves the complete provider credential as one operation.
func SetOpenSubtitles(dataDir, apiKey, username, password string) error {
	values := map[string]string{openSubtitlesAPIKey: apiKey, openSubtitlesUsername: username, openSubtitlesPassword: password}
	for key, value := range values {
		spec, _ := find(key)
		if err := validate(spec, value); err != nil || value == "" {
			return fmt.Errorf("invalid %s", key)
		}
	}
	configurationMutationMu.Lock()
	defer configurationMutationMu.Unlock()
	regular, secrets, err := readStoredConfiguration(dataDir)
	if err != nil {
		return err
	}
	for key, value := range values {
		secrets[key] = value
	}
	if err := validateStoredConfiguration(regular, secrets); err != nil {
		return err
	}
	return persistStoredConfiguration(dataDir, regular, secrets, false, true)
}

// DeleteOpenSubtitles removes the complete provider credential as one operation.
func DeleteOpenSubtitles(dataDir string) error {
	configurationMutationMu.Lock()
	defer configurationMutationMu.Unlock()
	regular, secrets, err := readStoredConfiguration(dataDir)
	if err != nil {
		return err
	}
	for _, key := range openSubtitlesKeys {
		delete(secrets, key)
	}
	if err := validateStoredConfiguration(regular, secrets); err != nil {
		return err
	}
	return persistStoredConfiguration(dataDir, regular, secrets, false, true)
}
