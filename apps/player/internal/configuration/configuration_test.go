package configuration_test

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/configurationtest"
)

func TestSharedConfigurationContract(t *testing.T) {
	configurationtest.Run(t, configuration.Load, configuration.Set, configuration.Delete,
		configuration.TrustedOrigin, (*configuration.Snapshot).UpdateGUI,
		configuration.Default, configuration.GUI, configuration.YAML, configuration.Environment, "38127")
}

func TestPlayerSupporterProductionDefaultsAndExplicitOverride(t *testing.T) {
	t.Parallel()
	loaded, err := configuration.Load(t.TempDir(), "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	if loaded.String("supporter.activation_url") != configuration.DefaultSupporterActivationURL || loaded.String("supporter.url") != configuration.DefaultSupportURL ||
		loaded.Source("supporter.activation_url") != configuration.Default || loaded.Source("supporter.url") != configuration.Default {
		t.Fatal("Player Supporter production defaults were not applied")
	}
	override := map[string]string{"KINOSAIL_SUPPORTER_ACTIVATION_URL": "http://127.0.0.1:9191/v1/supporters/activate", "KINOSAIL_SUPPORT_URL": "https://support.example/player"}
	loaded, err = configuration.Load(t.TempDir(), "", func(key string) (string, bool) { value, ok := override[key]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	if loaded.String("supporter.activation_url") != override["KINOSAIL_SUPPORTER_ACTIVATION_URL"] || loaded.String("supporter.url") != override["KINOSAIL_SUPPORT_URL"] {
		t.Fatal("explicit Supporter URLs were replaced by production defaults")
	}
}
