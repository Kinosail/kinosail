//go:build !windows

package commandtest

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const exitingHelperVariable = "KINOSAIL_CLI_EXIT_HELPER"

func ExitingHelperProcess(t *testing.T, main func()) {
	t.Helper()
	if os.Getenv(exitingHelperVariable) != "1" {
		return
	}
	HelperProcess(t, main)
	os.Exit(0)
}

type ServerProcess struct {
	Handler http.Handler
	DataDir string
	address string
	command *exec.Cmd
	output  *bytes.Buffer
	exited  chan struct{}
	exitErr error
}

func StartServerProcess(t *testing.T, helper string, environment ...string) *ServerProcess {
	t.Helper()
	process := &ServerProcess{DataDir: t.TempDir(), address: freeAddress(t), output: new(bytes.Buffer), exited: make(chan struct{})}
	process.command = exec.CommandContext(context.WithoutCancel(t.Context()), os.Args[0], "-test.run=^"+helper+"$", "--") //nolint:gosec // The executable is the current Go test binary.
	process.command.Env = append(append(os.Environ(),
		"KINOSAIL_CLI_HELPER=1", exitingHelperVariable+"=1", "KINOSAIL_LISTEN="+process.address, "KINOSAIL_TLS_ENABLED=false",
		"KINOSAIL_DATA_DIR="+process.DataDir, "KINOSAIL_FFMPEG="+fakeFFmpeg(t),
	), environment...)
	process.command.Stdout, process.command.Stderr = process.output, process.output
	if err := process.command.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(process.exited)
		process.exitErr = process.command.Wait()
	}()
	t.Cleanup(func() {
		_ = process.command.Process.Kill()
		<-process.exited
		if t.Failed() {
			t.Logf("server output: %s", process.output.String())
		}
	})
	target := &url.URL{Scheme: "http", Host: process.address}
	process.Handler = &httputil.ReverseProxy{Rewrite: func(request *httputil.ProxyRequest) { request.SetURL(target) }}
	process.waitReady(t)
	return process
}

func (process *ServerProcess) Terminate(t *testing.T) {
	t.Helper()
	if err := process.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-process.exited:
		if process.exitErr != nil {
			t.Fatalf("server exit after SIGTERM = %v", process.exitErr)
		}
	case <-time.After(time.Minute):
		t.Fatal("server did not exit after SIGTERM")
	}
}

func (process *ServerProcess) waitReady(t *testing.T) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		select {
		case <-process.exited:
			t.Fatalf("server exited before it was ready: %v", process.exitErr)
		default:
		}
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+process.address+"/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}
		if response, err := client.Do(request); err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
	}
	t.Fatal("server did not become ready")
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func fakeFFmpeg(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil { //nolint:gosec // The owner-only test fixture must be executable.
		t.Fatal(err)
	}
	return path
}
