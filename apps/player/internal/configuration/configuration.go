// Package configuration resolves typed Server configuration from GUI state, YAML, and the environment.
package configuration

import "github.com/MikeO7/kinosail/packages/configurationcore"

type Source string

const (
	Default     Source = "default"
	GUI         Source = "gui"
	YAML        Source = "yaml"
	Environment Source = "environment"
)

type kind string

const (
	text     kind = "text"
	boolean  kind = "boolean"
	duration kind = "duration"
	number   kind = "number"
	list     kind = "list"
)

type Spec struct {
	Key, Env, Default string
	Kind              kind
	Secret, Restart   bool
}

const (
	DefaultSupporterActivationURL = "https://kinosail-supporter-prod.pvw-7m4q2x9.workers.dev/v1/supporters/activate"
	DefaultSupportURL             = "https://buy.polar.sh/polar_cl_qnIK97BxBRJpKtqw0na3QocqgA3IyY0KlM6wZ36YiP8"
)

var specs = append(applicationSpecs(configurationcore.CommonApplicationFields("127.0.0.1:38127", DefaultSupportURL)),
	Spec{"integrations.google_cast.app_id", "KINOSAIL_GOOGLE_CAST_APP_ID", "", text, false, true})

func applicationSpecs(fields []configurationcore.ApplicationField) []Spec {
	result := make([]Spec, len(fields))
	for index, field := range fields {
		result[index] = Spec{field.Key, field.Env, field.Default, kind(field.Kind), field.Secret, field.Restart}
		if field.Key == "supporter.activation_url" {
			result[index].Default = DefaultSupporterActivationURL
		}
	}
	return result
}

type value struct {
	raw    string
	source Source
}

//nolint:recvcheck // Read access is value-based; UpdateGUI initializes and mutates the stored map.
type Snapshot struct{ values map[string]value }

type PublicValue struct {
	Key        string `json:"key"`
	Env        string `json:"env"`
	Value      string `json:"value,omitempty"`
	Source     Source `json:"source"`
	Secret     bool   `json:"secret"`
	Restart    bool   `json:"restartRequired"`
	Configured bool   `json:"configured"`
}

func Load(dataDir, yamlPath string, lookup func(string) (string, bool)) (Snapshot, error) {
	return loadConfiguration(dataDir, yamlPath, lookup)
}

func retiredConfigurationKey(key string) bool {
	return key == "integrations.subdl.url" || key == "integrations.subdl.api_key" || key == "integrations.subdl.api_key_file"
}

func discardRetiredConfiguration(values map[string]string) {
	for key := range values {
		if retiredConfigurationKey(key) {
			delete(values, key)
		}
	}
}

func find(key string) (Spec, bool) {
	for _, spec := range specs {
		if spec.Key == key {
			return spec, true
		}
	}
	return Spec{}, false
}
