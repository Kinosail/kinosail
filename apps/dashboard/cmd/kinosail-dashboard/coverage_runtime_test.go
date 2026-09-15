package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/database"
)

func coverageRuntimeEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"LISTEN", "DATA_DIR", "PUBLIC_URL", "SECURE_COOKIES", "PROBE_INTERVAL", "PUBLIC_PROBE_HOSTS", "TRUSTED_HOSTS"} {
		t.Setenv("KINOSAIL_DASHBOARD_"+name, "")
	}
	t.Setenv("KINOSAIL_SUPPORTER_ACTIVATION_URL", "")
	t.Setenv("KINOSAIL_SUPPORTER_URL", "")
}

func TestCoverageRuntimeValidation(t *testing.T) {
	for _, test := range []struct{ name, value string }{
		{"KINOSAIL_DASHBOARD_LISTEN", strings.Repeat("a", 257)},
		{"KINOSAIL_DASHBOARD_LISTEN", "invalid"},
		{"KINOSAIL_DASHBOARD_DATA_DIR", "/"},
		{"KINOSAIL_DASHBOARD_SECURE_COOKIES", "invalid"},
		{"KINOSAIL_DASHBOARD_PUBLIC_URL", "ftp://invalid.test"},
		{"KINOSAIL_DASHBOARD_PROBE_INTERVAL", "1s"},
		{"KINOSAIL_DASHBOARD_PUBLIC_PROBE_HOSTS", "*"},
		{"KINOSAIL_DASHBOARD_TRUSTED_HOSTS", "*"},
		{"KINOSAIL_SUPPORTER_URL", strings.Repeat("a", 2049)},
	} {
		t.Run(test.name+"/"+test.value[:min(8, len(test.value))], func(t *testing.T) {
			coverageRuntimeEnvironment(t)
			t.Setenv(test.name, test.value)
			if _, err := loadRuntimeConfig(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	coverageRuntimeEnvironment(t)
	t.Setenv("KINOSAIL_DASHBOARD_PROBE_INTERVAL", "20s")
	if got, err := loadRuntimeConfig(); err != nil || got.probeInterval != 20*time.Second {
		t.Fatalf("valid interval = %v, %v", got.probeInterval, err)
	}
}

func TestCoverageRuntimeHostLimits(t *testing.T) {
	if _, err := defaultPublicURL("invalid"); err == nil {
		t.Fatal("invalid listen address accepted")
	}
	for _, parse := range []func(string) ([]string, error){parseTrustedHosts, parsePublicHosts} {
		if _, err := parse(strings.Repeat("example.test,", 51)); err == nil {
			t.Fatal("oversized host list accepted")
		}
	}
	for _, host := range []string{"", strings.Repeat("a", 254), "a..test", strings.Repeat("a", 64) + ".test"} {
		if validHostname(host) {
			t.Fatalf("invalid hostname accepted: %q", host)
		}
	}
}

func TestCoverageOpenApplicationFailures(t *testing.T) {
	for _, mode := range []string{"directory", "board", "owner", "supporter"} {
		t.Run(mode, func(t *testing.T) {
			config := runtimeConfig{dataDir: t.TempDir(), probeInterval: time.Hour}
			switch mode {
			case "directory":
				config.dataDir = filepath.Join(config.dataDir, "file")
				if err := os.WriteFile(config.dataDir, []byte("data"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "supporter":
				config.activationURL = "ftp://invalid.test"
			default:
				coverageSeedInvalidDocument(t, config.dataDir, mode+".json")
			}
			if app, err := openApplication(t.Context(), config); err == nil {
				_ = app.store.Close()
				t.Fatal("invalid app state accepted")
			}
		})
	}
}

func coverageSeedInvalidDocument(t *testing.T, directory, name string) {
	t.Helper()
	store, err := database.Open(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveJSON(t.Context(), name, map[string]string{"invalid": "state"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageHealthcheckStatuses(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(status) }))
		address := strings.TrimPrefix(provider.URL, "http://")
		err := healthcheck(t.Context(), address)
		provider.Close()
		if (err == nil) != (status == http.StatusOK) {
			t.Fatalf("health status %d = %v", status, err)
		}
	}
	for _, address := range []string{"invalid", "bad\nname:80"} {
		if err := healthcheck(t.Context(), address); err == nil {
			t.Fatal("invalid health address accepted")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := healthcheck(ctx, net.JoinHostPort("0.0.0.0", "1")); err == nil {
		t.Fatal("canceled healthcheck accepted")
	}
}
