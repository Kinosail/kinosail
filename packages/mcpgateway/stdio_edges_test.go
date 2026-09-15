//go:build !windows

package mcpgateway

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func listenStdioTestSocket(t *testing.T, dataDir string) net.Listener {
	t.Helper()
	path, err := StdioSocketPath(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := new(net.ListenConfig).Listen(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(path)
	})
	return listener
}

func TestServeStdioJoinsCompletedInputAndOutput(t *testing.T) {
	dataDir := t.TempDir()
	listener := listenStdioTestSocket(t, dataDir)
	done := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		reader := bufio.NewReader(connection)
		_, err = reader.ReadString('\n')
		if err == nil {
			_, err = io.WriteString(connection, "OK\n")
		}
		if err == nil {
			_, err = io.Copy(io.Discard, reader)
		}
		time.Sleep(20 * time.Millisecond)
		_ = connection.Close()
		done <- err
	}()
	if err := ServeStdio(t.Context(), dataDir, "", strings.NewReader(""), io.Discard); err != nil {
		t.Fatalf("ServeStdio = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("socket server = %v", err)
	}
}

func TestServeStdioReturnsOutputWhenInputIsStillOpen(t *testing.T) {
	dataDir := t.TempDir()
	listener := listenStdioTestSocket(t, dataDir)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_, _ = bufio.NewReader(connection).ReadString('\n')
			_, _ = io.WriteString(connection, "OK\n")
			_ = connection.Close()
		}
	}()
	input, release := io.Pipe()
	err := ServeStdio(t.Context(), dataDir, "", input, io.Discard)
	_ = release.Close()
	if err != nil {
		t.Fatalf("ServeStdio output completion = %v", err)
	}
}

func TestOpenStdioFailuresReturnNoRelayOrConnection(t *testing.T) {
	if relay, err := OpenStdioRelay(t.Context(), t.TempDir(), ""); err == nil || relay != nil {
		t.Fatalf("missing relay = %#v, %v", relay, err)
	}
	if connection, reader, err := openMCPStdio(nil, t.TempDir(), ""); err == nil || connection != nil || reader != nil { //nolint:staticcheck // The nil-context boundary is intentional.
		t.Fatalf("nil context = %#v, %#v, %v", connection, reader, err)
	}
	if connection, reader, err := openMCPStdio(t.Context(), "", ""); err == nil || connection != nil || reader != nil {
		t.Fatalf("empty data directory = %#v, %#v, %v", connection, reader, err)
	}
}

func TestOpenStdioUsesSafeDefaultHandshakeFailure(t *testing.T) {
	dataDir := t.TempDir()
	listener := listenStdioTestSocket(t, dataDir)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_, _ = bufio.NewReader(connection).ReadString('\n')
			_ = connection.Close()
		}
	}()
	connection, reader, err := openMCPStdio(t.Context(), dataDir, "")
	if err == nil || err.Error() != "could not select the MCP Owner Profile" || connection != nil || reader != nil {
		t.Fatalf("empty handshake = %#v, %#v, %v", connection, reader, err)
	}
}

func TestStartStdioHostRejectsNonSocketAndCancelledStaleSocket(t *testing.T) {
	principals := &testPrincipals{values: map[string]Principal{"owner": {ID: "owner", Owner: true}}}
	dataDir := t.TempDir()
	path, _ := StdioSocketPath(dataDir)
	if err := os.WriteFile(path, []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := StartStdioHost(t.Context(), dataDir, stdioFixture(t, principals), principals); err == nil || !strings.Contains(err.Error(), "not a socket") {
		t.Fatalf("regular socket path = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	listener, err := new(net.ListenConfig).Listen(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	listener.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := StartStdioHost(ctx, dataDir, stdioFixture(t, principals), principals); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled stale socket = %v", err)
	}
	_ = os.Remove(path)
}

func TestStdioHostCancellationRemovesOwnedSocket(t *testing.T) {
	principals := &testPrincipals{values: map[string]Principal{"owner": {ID: "owner", Owner: true}}}
	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	if err := StartStdioHost(ctx, dataDir, stdioFixture(t, principals), principals); err != nil {
		t.Fatal(err)
	}
	path, _ := StdioSocketPath(dataDir)
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		_, err := os.Lstat(path)
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("owned socket was not removed: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStartStdioHostReportsFilesystemFailures(t *testing.T) { //nolint:gocognit // The score of 17 remains below the repository ceiling of 22 for the filesystem failure matrix.
	principals := &testPrincipals{values: map[string]Principal{"owner": {ID: "owner", Owner: true}}}
	for _, test := range []struct {
		name  string
		setup func(*testing.T, string, string)
		mode  os.FileMode
	}{
		{"stale removal", func(t *testing.T, _, path string) {
			listener, err := new(net.ListenConfig).Listen(t.Context(), "unix", path)
			if err != nil {
				t.Fatal(err)
			}
			listener.(*net.UnixListener).SetUnlinkOnClose(false)
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
		}, 0o500},
		{"stat", func(*testing.T, string, string) {}, 0o000},
		{"listen", func(*testing.T, string, string) {}, 0o500},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, err := os.MkdirTemp("/tmp", "mcp-stdio-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(root) })
			t.Setenv("TMPDIR", root)
			dataDir := "private-data"
			path, err := StdioSocketPath(dataDir)
			if err != nil {
				t.Fatal(err)
			}
			test.setup(t, root, path)
			if err := os.Chmod(root, test.mode); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Chmod(root, 0o700) }() //nolint:gosec // Directory cleanup requires execute permission.
			err = StartStdioHost(t.Context(), dataDir, stdioFixture(t, principals), principals)
			if err == nil {
				t.Fatal("filesystem failure started a host")
			}
		})
	}
}

func TestServeStdioConnectionReportsOwnerAndWriteFailures(t *testing.T) {
	server, client := net.Pipe()
	done := make(chan struct{})
	go func() {
		serveMCPStdioConnection(t.Context(), server, stdioFixture(t, &testPrincipals{values: map[string]Principal{}}), &testPrincipals{values: map[string]Principal{}})
		close(done)
	}()
	_, _ = io.WriteString(client, "missing\n")
	response, _ := bufio.NewReader(client).ReadString('\n')
	_ = client.Close()
	<-done
	if !strings.Contains(response, "Owner Profile was not found") {
		t.Fatalf("missing owner response = %q", response)
	}
	owner := Principal{ID: "owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{"owner": owner}}
	connection := &writeFailureConn{Reader: bytes.NewBufferString("owner\n")}
	serveMCPStdioConnection(t.Context(), connection, stdioFixture(t, principals), principals)
	if connection.writes != 1 {
		t.Fatalf("handshake write attempts = %d", connection.writes)
	}
}
