package configuration

import (
	"errors"

	"github.com/MikeO7/kinosail/packages/configurationcore"
	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

func loadConfiguration(dataDir, yamlPath string, lookup func(string) (string, bool)) (Snapshot, error) {
	loaded, err := configurationLoader().Load(dataDir, yamlPath, lookup)
	if err != nil {
		return Snapshot{}, err
	}
	values := make(map[string]value, len(loaded))
	for key, item := range loaded {
		values[key] = value{item.Raw, Source(item.Source)}
	}
	configured := Snapshot{values}
	if err := validateConfiguredSnapshot(configured); err != nil {
		return Snapshot{}, err
	}
	return configured, nil
}

func configurationLoader() configurationcore.Loader {
	return configurationcore.Loader{
		Schema: configurationSchema(),
		ReadStored: func(dataDir string) (map[string]string, map[string]string, error) {
			configurationMutationMu.Lock()
			defer configurationMutationMu.Unlock()
			return readStoredConfiguration(dataDir)
		},
		ReadSecret: func(path string) ([]byte, error) { return privatefile.Read(path, 16<<10) },
	}
}

func configurationSchema() configurationcore.Schema {
	fields := make([]configurationcore.Field, len(specs))
	for index, spec := range specs {
		fields[index] = configurationcore.Field{Key: spec.Key, Env: spec.Env, Default: spec.Default, Kind: configurationcore.Kind(spec.Kind), Secret: spec.Secret}
	}
	return configurationcore.Schema{Fields: fields, Validate: validateConfigurationValue}
}

func validateConfigurationValue(key, raw string) error {
	spec, _ := find(key)
	return validate(spec, raw)
}

func validateConfiguredSnapshot(configured Snapshot) error {
	if configured.Source("dlna.enabled") != Default && configured.Bool("dlna.enabled") && configured.String("dlna.url") == "" {
		return errors.New("dlna.url is required when DLNA is enabled")
	}
	validators := []func(Snapshot) error{configurationcore.ValidateMCP[Snapshot], validateRemoteAccess, validateSCIM, validateFederatedOIDC, validateFederatedSAML, validateOpenSubtitles, validateSubSource, validateTrustedHTTPS, validateJellyfinCompatibility, validateSupporter}
	for _, validator := range validators {
		if err := validator(configured); err != nil {
			return err
		}
	}
	return nil
}

func validateFederatedOIDC(configured Snapshot) error {
	return federation.ValidateOIDC(configured.String)
}

func validateFederatedSAML(configured Snapshot) error {
	return federation.ValidateSAML(configured.String)
}
