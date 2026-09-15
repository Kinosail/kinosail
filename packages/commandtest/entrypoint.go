package commandtest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func Entrypoint(t *testing.T) {
	for name, test := range map[string]struct {
		args       []string
		wantOutput string
		wantExit   bool
	}{
		"version": {[]string{"version"}, "dev\n", false},
		"unknown": {[]string{"unknown"}, "command failed", true},
		"usage":   {[]string{"version", "extra"}, "command failed", true},
	} {
		t.Run(name, func(t *testing.T) {
			//nolint:gosec // G204: the executable is the current Go test binary, not external input.
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestCommandLineHelperProcess", "--")
			command.Args = append(command.Args, test.args...)
			command.Env = append(os.Environ(), "KINOSAIL_CLI_HELPER=1", "KINOSAIL_DATA_DIR="+t.TempDir())
			output, err := command.CombinedOutput()
			if (err != nil) != test.wantExit || !strings.Contains(string(output), test.wantOutput) {
				t.Fatalf("args %v = %q, %v", test.args, output, err)
			}
		})
	}
}

func ListenFailure(t *testing.T) {
	//nolint:gosec // G204: the executable is the current Go test binary, not external input.
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestCommandLineHelperProcess", "--")
	command.Env = append(os.Environ(), "KINOSAIL_CLI_HELPER=1", "KINOSAIL_DATA_DIR="+t.TempDir(), "KINOSAIL_LISTEN=invalid")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "server failed") {
		t.Fatalf("listen failure = %q, %v", output, err)
	}
}

func HelperProcess(t *testing.T, main func()) {
	t.Helper()
	if os.Getenv("KINOSAIL_CLI_HELPER") != "1" {
		return
	}
	separator := 0
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index + 1
			break
		}
	}
	os.Args = append([]string{"kinosail"}, os.Args[separator:]...)
	main()
}
