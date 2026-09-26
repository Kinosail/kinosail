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

const DefaultSupporterActivationURL = "https://kinosail-supporter-prod.pvw-7m4q2x9.workers.dev/v1/supporters/activate"

var specs = append(applicationSpecs(configurationcore.CommonApplicationFields("127.0.0.1:38128", "https://github.com/Kinosail/kinosail/tree/main/apps/subtitles")), []Spec{
	{"binaries.tesseract", "KINOSAIL_TESSERACT", "tesseract", text, false, true},
	{"binaries.whisper", "KINOSAIL_WHISPER", "whisper-cli", text, false, true},
	{"subtitles.transcription_model", "KINOSAIL_TRANSCRIPTION_MODEL", "", text, false, true},
	{"subtitles.transcription_architecture", "KINOSAIL_TRANSCRIPTION_ARCHITECTURE", "small", text, false, true},
	{"integrations.subdl.url", "KINOSAIL_SUBDL_URL", "", text, false, true},
	{"integrations.subdl.api_key", "KINOSAIL_SUBDL_API_KEY", "", text, true, true},
	{"integrations.subsource.url", "KINOSAIL_SUBSOURCE_URL", "", text, false, true},
	{"integrations.subsource.api_key", "KINOSAIL_SUBSOURCE_API_KEY", "", text, true, true},
	{"integrations.subsource.personal_use", "KINOSAIL_SUBSOURCE_PERSONAL_USE", "", boolean, false, true},
	{"integrations.opensubtitles.url", "KINOSAIL_OPENSUBTITLES_URL", "", text, false, true},
	{"integrations.opensubtitles.api_key", "KINOSAIL_OPENSUBTITLES_API_KEY", "", text, true, true},
	{"integrations.opensubtitles.username", "KINOSAIL_OPENSUBTITLES_USERNAME", "", text, true, true},
	{"integrations.opensubtitles.password", "KINOSAIL_OPENSUBTITLES_PASSWORD", "", text, true, true},
}...)

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

func find(key string) (Spec, bool) {
	for _, spec := range specs {
		if spec.Key == key {
			return spec, true
		}
	}
	return Spec{}, false
}
