//go:build !windows

package mcpgateway

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestInitializeStdioReportsDeadlineWriteAndResetFailures(t *testing.T) {
	want := errors.New("deadline failed")
	initial := &handshakeConn{Reader: bytes.NewBufferString("OK\n"), deadlineErrors: []error{want}}
	if reader, err := initializeMCPStdio(initial, "owner"); !errors.Is(err, want) || reader != nil || initial.writes != 0 {
		t.Fatalf("initial deadline = %#v, %v writes=%d", reader, err, initial.writes)
	}
	write := &handshakeConn{Reader: bytes.NewBufferString("OK\n"), writeErr: errors.New("write failed")}
	if reader, err := initializeMCPStdio(write, "owner"); err == nil || reader != nil || write.writes != 1 {
		t.Fatalf("selector write = %#v, %v writes=%d", reader, err, write.writes)
	}
	reset := &handshakeConn{Reader: bytes.NewBufferString("OK\n"), deadlineErrors: []error{nil, want}}
	if reader, err := initializeMCPStdio(reset, "owner"); !errors.Is(err, want) || reader != nil || reset.deadlines != 2 {
		t.Fatalf("deadline reset = %#v, %v deadlines=%d", reader, err, reset.deadlines)
	}
}

func TestStdioSocketSecurityRollbackAndOwnedRemoval(t *testing.T) {
	listener := &failureListener{}
	missing := "/tmp/kinosail-mcp-missing-security-path"
	_ = os.Remove(missing)
	if err := secureMCPStdioSocket(listener, missing); err == nil || listener.closes != 1 {
		t.Fatalf("socket security rollback = %v closes=%d", err, listener.closes)
	}
	root, err := os.MkdirTemp("/tmp", "mcp-remove-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	socket := root + "/owned.sock"
	owned, err := new(net.ListenConfig).Listen(t.Context(), "unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	owned.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	removeMCPStdioSocket(socket)
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatalf("owned socket remained: %v", err)
	}
	regular := root + "/regular"
	if err := os.WriteFile(regular, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	removeMCPStdioSocket(regular)
	if _, err := os.Stat(regular); err != nil {
		t.Fatalf("non-socket was removed: %v", err)
	}
}

type writeFailureConn struct {
	io.Reader
	writes int
}

func (connection *writeFailureConn) Write([]byte) (int, error) {
	connection.writes++
	return 0, errors.New("write failed")
}
func (*writeFailureConn) Close() error                     { return nil }
func (*writeFailureConn) LocalAddr() net.Addr              { return testAddr("local") }
func (*writeFailureConn) RemoteAddr() net.Addr             { return testAddr("remote") }
func (*writeFailureConn) SetDeadline(time.Time) error      { return nil }
func (*writeFailureConn) SetReadDeadline(time.Time) error  { return nil }
func (*writeFailureConn) SetWriteDeadline(time.Time) error { return nil }

type testAddr string

func (address testAddr) Network() string { return string(address) }
func (address testAddr) String() string  { return string(address) }

type handshakeConn struct {
	io.Reader
	writeErr       error
	deadlineErrors []error
	writes         int
	deadlines      int
}

func (connection *handshakeConn) Write(data []byte) (int, error) {
	connection.writes++
	if connection.writeErr != nil {
		return 0, connection.writeErr
	}
	return len(data), nil
}
func (*handshakeConn) Close() error                     { return nil }
func (*handshakeConn) LocalAddr() net.Addr              { return testAddr("local") }
func (*handshakeConn) RemoteAddr() net.Addr             { return testAddr("remote") }
func (*handshakeConn) SetReadDeadline(time.Time) error  { return nil }
func (*handshakeConn) SetWriteDeadline(time.Time) error { return nil }
func (connection *handshakeConn) SetDeadline(time.Time) error {
	index := connection.deadlines
	connection.deadlines++
	if index < len(connection.deadlineErrors) {
		return connection.deadlineErrors[index]
	}
	return nil
}

type failureListener struct{ closes int }

func (*failureListener) Accept() (net.Conn, error) { return nil, errors.New("unused") }
func (listener *failureListener) Close() error {
	listener.closes++
	return nil
}
func (*failureListener) Addr() net.Addr { return testAddr("listener") }
