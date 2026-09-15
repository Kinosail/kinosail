package configuration

import "fmt"

// SetSubSource saves the API key and personal-use acceptance as one operation.
func SetSubSource(dataDir, apiKey string) error {
	keySpec, _ := find(subSourceAPIKey)
	if err := validate(keySpec, apiKey); err != nil || apiKey == "" {
		return fmt.Errorf("invalid %s", subSourceAPIKey)
	}
	configurationMutationMu.Lock()
	defer configurationMutationMu.Unlock()
	regular, secrets, err := readStoredConfiguration(dataDir)
	if err != nil {
		return err
	}
	secrets[subSourceAPIKey], regular[subSourcePersonalUse] = apiKey, "true"
	if err := validateStoredConfiguration(regular, secrets); err != nil {
		return err
	}
	return persistStoredConfiguration(dataDir, regular, secrets, true, true)
}

// DeleteSubSource removes the API key and personal-use acceptance together.
func DeleteSubSource(dataDir string) error {
	configurationMutationMu.Lock()
	defer configurationMutationMu.Unlock()
	regular, secrets, err := readStoredConfiguration(dataDir)
	if err != nil {
		return err
	}
	delete(secrets, subSourceAPIKey)
	delete(regular, subSourcePersonalUse)
	if err := validateStoredConfiguration(regular, secrets); err != nil {
		return err
	}
	return persistStoredConfiguration(dataDir, regular, secrets, true, true)
}
