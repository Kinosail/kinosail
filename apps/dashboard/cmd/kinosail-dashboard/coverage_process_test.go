package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/commandtest"
)

const coverageCommandVariable = "KINOSAIL_DASHBOARD_TEST_COMMAND"

func TestCoverageDashboardMainSubprocess(_ *testing.T) {
	command := os.Getenv(coverageCommandVariable)
	if command == "" {
		return
	}
	os.Args = append([]string{"kinosail-dashboard"}, strings.Fields(command)...)
	main()
}

func TestCoverageMainProcessExitPaths(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) }))
	t.Cleanup(provider.Close)
	listen := strings.TrimPrefix(provider.URL, "http://")
	for _, scenario := range []struct {
		name, command, listen string
		exit                  int
	}{
		{"command", "unknown", "127.0.0.1:38400", 1},
		{"config", "serve", "invalid", 1},
		{"health failure", "healthcheck", "127.0.0.1:1", 1},
		{"health success", "healthcheck", listen, 0},
		{"storage", "serve", "127.0.0.1:38400", 1},
		{"listen", "serve", listen, 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			coverageRuntimeEnvironment(t)
			t.Setenv("KINOSAIL_DASHBOARD_LISTEN", scenario.listen)
			directory := t.TempDir()
			if scenario.name == "storage" {
				directory = filepath.Join(directory, "file")
				if err := os.WriteFile(directory, []byte("data"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("KINOSAIL_DASHBOARD_DATA_DIR", directory)
			command := coverageMainCommand(t, scenario.command)
			output, err := command.CombinedOutput()
			assertCoverageMainExit(t, scenario.exit, output, err)
		})
	}
}

func assertCoverageMainExit(t *testing.T, want int, output []byte, err error) {
	t.Helper()
	if want == 0 {
		if err != nil {
			t.Fatalf("main failed: %v\n%s", err, output)
		}
		return
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != want {
		t.Fatalf("main exit = %v, want %d\n%s", err, want, output)
	}
}

func coverageMainCommand(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	return commandtest.CoveredChild(t, "TestCoverageDashboardMainSubprocess", coverageCommandVariable+"="+mode)
}
