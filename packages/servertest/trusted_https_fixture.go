package servertest

import (
	"net/http"
	"testing"
)

// BindTrustedHTTPS keeps construction and reloads on the same real app configuration loader.
func BindTrustedHTTPS[Config interface{ String(string) string }](fixture TrustedHTTPSFixture, load func(string, string, func(string) (string, bool)) (Config, error), newHandler func(string, Config) http.Handler) TrustedHTTPSFixture {
	read := func(directory string) (Config, error) {
		return load(directory, "", func(string) (string, bool) { return "", false })
	}
	fixture.New = func(t *testing.T, directory string) http.Handler {
		t.Helper()
		configured, err := read(directory)
		if err != nil {
			t.Fatal(err)
		}
		return newHandler(directory, configured)
	}
	fixture.Load = func(directory string) (string, string, error) {
		configured, err := read(directory)
		return configured.String("tls.duckdns"), configured.String("listen"), err
	}
	return fixture
}
