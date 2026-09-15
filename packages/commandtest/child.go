package commandtest

import (
	"flag"
	"os"
	"os/exec"
	"regexp"
	"testing"
)

// CoveredChild constructs an unstarted command for one exact test in the current
// test binary. Environment overrides use exec.Cmd's last-value-wins semantics.
// Both normal test completion and os.Exit paths contribute child coverage.
func CoveredChild(t *testing.T, target string, environment ...string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G204: execute only the current Go test binary, never external input.
	command := exec.CommandContext(t.Context(), executable, "-test.run=^"+regexp.QuoteMeta(target)+"$")
	command.Env = append(os.Environ(), environment...)
	if directory := flag.Lookup("test.gocoverdir"); directory != nil && directory.Value.String() != "" {
		command.Args = append(command.Args, "-test.gocoverdir="+directory.Value.String())
		command.Env = append(command.Env, "GOCOVERDIR="+directory.Value.String())
	}
	return command
}
