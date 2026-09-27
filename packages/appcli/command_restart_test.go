package appcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/servertransport"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func TestRunWaitsForActiveRequestBeforeExiting(t *testing.T) { //nolint:cyclop,funlen,gocognit // The subprocess handshake verifies the real signal and request lifecycle.
	if os.Getenv("KINOSAIL_LIFECYCLE_HELPER") == "1" {
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(os.Stdout, listener.Addr().String())
		server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(os.Stdout, "active")
			_, _ = os.Stdin.Read(make([]byte, 1))
			_, _ = writer.Write([]byte("finished"))
		}), ReadHeaderTimeout: time.Second}
		os.Exit(run(testSettings{"remote.mode": "off"}, []os.Signal{syscall.SIGTERM},
			func(context.Context, *remoteaccess.Manager, *trustedhttps.Manager) *http.Server { return server },
			func(handler http.Handler) http.Handler { return handler },
			func(Settings) (*remoteaccess.Manager, error) { return nil, nil },
			func(Settings) (*trustedhttps.Manager, error) { return nil, nil },
			func(got *http.Server, _ servertransport.TLSConfig) error { return got.Serve(listener) }))
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRunWaitsForActiveRequestBeforeExiting$") //nolint:gosec // Reexecutes this test binary with a fixed test selector.
	command.Env = append(os.Environ(), "KINOSAIL_LIFECYCLE_HELPER=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()
	lines := bufio.NewScanner(stdout)
	if !lines.Scan() {
		t.Fatal("child did not report listener")
	}
	address := lines.Text()
	result := make(chan error, 1)
	go func() {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+address, nil)
		if err != nil {
			result <- err
			return
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			result <- err
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err == nil && string(body) != "finished" {
			err = fmt.Errorf("response body = %q", body)
		}
		result <- err
	}()
	if !lines.Scan() || lines.Text() != "active" {
		t.Fatal("request did not become active")
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	select {
	case err := <-exited:
		t.Fatalf("server exited before active request completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := stdin.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active request did not finish")
	}
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit after request completed")
	}
}
