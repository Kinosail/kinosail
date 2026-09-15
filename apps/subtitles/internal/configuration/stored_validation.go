package configuration

import (
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/configurationcore"
	"github.com/MikeO7/kinosail/packages/federation"
)

func readStoredConfiguration(dataDir string) (map[string]string, map[string]string, error) {
	if err := storedConfigurationTransaction.Recover(dataDir); err != nil {
		return nil, nil, err
	}
	regular, err := readJSON(filepath.Join(dataDir, "configuration.json"))
	if err != nil {
		return nil, nil, err
	}
	secrets, err := readJSON(filepath.Join(dataDir, "secrets.json"))
	if err != nil {
		return nil, nil, err
	}
	return regular, secrets, nil
}

func validateStoredConfiguration(regular, secrets map[string]string) error { //nolint:cyclop,gocognit // Stored configuration is validated by source file and then by cross-field policy.
	loaded, err := (configurationcore.Loader{Schema: configurationSchema()}).ResolveStored(regular, secrets)
	if err != nil {
		return err
	}
	values := make(map[string]value, len(loaded))
	for key, item := range loaded {
		values[key] = value{item.Raw, Source(item.Source)}
	}
	configured := Snapshot{values}
	if err := validateSCIM(configured); err != nil {
		return err
	}
	if err := federation.ValidateOIDC(configured.String); err != nil {
		return err
	}
	if err := federation.ValidateSAML(configured.String); err != nil {
		return err
	} else if err := validateOpenSubtitles(configured); err != nil {
		return err
	} else if err := validateSubSource(configured); err != nil {
		return err
	}
	if err := validateTrustedHTTPS(configured); err != nil {
		return err
	}
	if err := validateJellyfinCompatibility(configured); err != nil {
		return err
	}
	return nil
}
