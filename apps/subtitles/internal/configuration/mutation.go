package configuration

import (
	"sync"

	"github.com/MikeO7/kinosail/packages/configurationcore"
	"github.com/MikeO7/kinosail/packages/federation"
)

var configurationMutationMu sync.Mutex

func federationSettings(dataDir string) federation.Settings {
	return federation.NewSettings(dataDir, &configurationMutationMu, validateFederationSetting, readStoredConfiguration, validateStoredConfiguration, persistStoredConfiguration)
}

func validateFederationSetting(key, value string) error {
	spec, _ := find(key)
	return validate(spec, value)
}

const (
	scimTokenKey      = configurationcore.SCIMTokenKey
	scimExpirationKey = configurationcore.SCIMExpirationKey
	configurationFile = "configuration.json"
	secretsFile       = "secrets.json"
)

var storedConfigurationTransaction = configurationcore.Transaction{
	RegularFile: configurationFile,
	SecretsFile: secretsFile,
	Parse:       parseJSON,
	Validate:    validateStoredConfiguration,
}

var storedConfigurationPair = configurationcore.Pair{
	RegularFile: configurationFile,
	SecretsFile: secretsFile,
	Transaction: storedConfigurationTransaction,
}

func configurationMutation() configurationcore.Mutation {
	return configurationcore.Mutation{
		Lock: &configurationMutationMu,
		Find: func(key string) (bool, bool) {
			spec, ok := find(key)
			return spec.Secret, ok
		},
		CoupledError:   coupledSettingError,
		Validate:       validateConfigurationValue,
		Read:           readStoredConfiguration,
		ValidateStored: validateStoredConfiguration,
		Persist:        persistStoredConfiguration,
	}
}

func Set(dataDir, key, raw string) error { return configurationMutation().Set(dataDir, key, raw) }
func Delete(dataDir, key string) error   { return configurationMutation().Delete(dataDir, key) }

// SetSCIM changes the token and its expiry as one validated configuration operation.
func SetSCIM(dataDir, token, expiresAt string) error {
	return configurationMutation().SetSCIM(dataDir, token, expiresAt)
}

// DeleteSCIM removes the token and expiry as one validated configuration operation.
func DeleteSCIM(dataDir string) error { return configurationMutation().DeleteSCIM(dataDir) }

func persistStoredConfiguration(dataDir string, regular, secrets map[string]string, writeRegular, writeSecrets bool) error {
	return storedConfigurationPair.Persist(dataDir, regular, secrets, writeRegular, writeSecrets)
}

func (snapshot *Snapshot) UpdateGUI(key, raw string, deleted bool) {
	if snapshot.values == nil {
		snapshot.values = make(map[string]value)
	}
	if deleted {
		spec, _ := find(key)
		snapshot.values[key] = value{spec.Default, Default}
		return
	}
	snapshot.values[key] = value{raw, GUI}
}
