package configuration_test

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/configurationtest"
)

func TestSharedConfigurationContract(t *testing.T) {
	configurationtest.Run(t, configuration.Load, configuration.Set, configuration.Delete,
		configuration.TrustedOrigin, (*configuration.Snapshot).UpdateGUI,
		configuration.Default, configuration.GUI, configuration.YAML, configuration.Environment, "38128")
}

func TestSupporterActivationDefaultsToProduction(t *testing.T) {
	configured, err := configuration.Load(t.TempDir(), "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	const want = "https://kinosail-supporter-prod.pvw-7m4q2x9.workers.dev/v1/supporters/activate"
	if got := configured.String("supporter.activation_url"); got != want || configured.Source("supporter.activation_url") != configuration.Default {
		t.Fatalf("default supporter activation = %q (%s), want %q", got, configured.Source("supporter.activation_url"), want)
	}
}

func TestSupporterPurchaseDefaultsToSubtitlesCheckout(t *testing.T) {
	configured, err := configuration.Load(t.TempDir(), "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	const want = "https://buy.polar.sh/polar_cl_HTsVz4n50S840QL5zKCZ0yTZe6HpVfHqagCtQ1LZ6sN"
	if got := configured.String("supporter.url"); got != want || configured.Source("supporter.url") != configuration.Default {
		t.Fatalf("default supporter purchase = %q (%s), want %q", got, configured.Source("supporter.url"), want)
	}
}
