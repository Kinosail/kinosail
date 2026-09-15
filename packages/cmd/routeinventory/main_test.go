package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunCoversUsageDiscoveryEncodingAndSuccess(t *testing.T) { //nolint:cyclop // One command contract protects every exit and output boundary.
	t.Parallel()
	var output, failures bytes.Buffer
	if code := run(nil, &output, &failures); code != 2 || !strings.Contains(failures.String(), "usage:") {
		t.Fatalf("usage code=%d output=%q", code, failures.String())
	}
	failures.Reset()
	if code := run([]string{"/missing-app", "/missing-packages"}, &output, &failures); code != 1 || failures.Len() == 0 {
		t.Fatalf("discovery code=%d output=%q", code, failures.String())
	}
	app := filepath.Join(t.TempDir(), "app")
	packages, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "app.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	failures.Reset()
	if code := run([]string{app, packages}, errorWriter{}, &failures); code != 1 || !strings.Contains(failures.String(), "write failed") {
		t.Fatalf("encoding code=%d output=%q", code, failures.String())
	}
	output.Reset()
	if code := run([]string{app, packages}, &output, &failures); code != 0 || output.Len() == 0 {
		t.Fatalf("success code=%d output=%q failure=%q", code, output.String(), failures.String())
	}
}

func TestMainDispatchesExitCode(t *testing.T) {
	previousArgs, previousStderr, previousExit := os.Args, os.Stderr, exit
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Args, os.Stderr, exit = previousArgs, previousStderr, previousExit
		_ = stderr.Close()
	})
	os.Args, os.Stderr = []string{"routeinventory"}, stderr
	code := 0
	exit = func(value int) { code = value }
	main()
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
}
