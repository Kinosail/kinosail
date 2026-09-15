package mcpgateway

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpStdioSocketName      = "kinosail-mcp.sock"
	mcpStdioSelectorLimit   = 128
	mcpStdioConnectionLimit = 32
)

// ServeStdio bridges stdin and stdout to the running Kinosail Server.
func ServeStdio(ctx context.Context, dataDir, profileID string, input io.Reader, output io.Writer) error { //nolint:cyclop // Validation, handshake, cancellation, and bidirectional bridging form one transport lifecycle.
	if ctx == nil || input == nil || output == nil {
		return errors.New("MCP STDIO requires a context, input, and output")
	}
	connection, reader, err := openMCPStdio(ctx, dataDir, profileID)
	if err != nil {
		return err
	}
	defer connection.Close()
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	inputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(connection, input)
		if closer, ok := connection.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		}
		inputDone <- copyErr
	}()
	_, outputErr := io.Copy(output, reader)
	select {
	case inputErr := <-inputDone:
		return errors.Join(inputErr, outputErr)
	default:
		return outputErr
	}
}

// OpenStdioRelay returns an authorized Server socket for a lightweight relay.
func OpenStdioRelay(ctx context.Context, dataDir, profileID string) (*os.File, error) {
	connection, _, err := openMCPStdio(ctx, dataDir, profileID)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	unixConnection := connection.(*net.UnixConn)
	return unixConnection.File()
}

func openMCPStdio(ctx context.Context, dataDir, profileID string) (net.Conn, *bufio.Reader, error) { //nolint:cyclop // Validation, dialing, and the bounded Owner handshake form one transport operation.
	if ctx == nil {
		return nil, nil, errors.New("MCP STDIO requires a context")
	}
	if err := validMCPStdioSelector(profileID); err != nil {
		return nil, nil, err
	}
	path, err := StdioSocketPath(dataDir)
	if err != nil {
		return nil, nil, err
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, nil, errors.New("running Kinosail Server MCP connection is unavailable")
	}
	failed := true
	defer func() {
		if failed {
			_ = connection.Close()
		}
	}()
	reader, err := initializeMCPStdio(connection, profileID)
	if err != nil {
		return nil, nil, err
	}
	failed = false
	return connection, reader, nil
}

func initializeMCPStdio(connection net.Conn, profileID string) (*bufio.Reader, error) {
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(connection, profileID+"\n"); err != nil {
		return nil, errors.New("could not select the MCP Owner Profile")
	}
	reader := bufio.NewReaderSize(connection, mcpStdioSelectorLimit+2)
	response, err := reader.ReadString('\n')
	if err != nil || response != "OK\n" {
		message := strings.TrimSpace(strings.TrimPrefix(response, "ERR "))
		if message == "" {
			message = "could not select the MCP Owner Profile"
		}
		return nil, errors.New(message)
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return reader, nil
}

// StartStdioHost starts the private local MCP socket.
func StartStdioHost(ctx context.Context, dataDir string, adapter *Gateway, principals PrincipalRepository) error { //nolint:cyclop,gocognit // Safe stale-socket recovery and listener ownership form one startup decision.
	path, err := StdioSocketPath(dataDir)
	if err != nil || adapter == nil || principals == nil {
		return errors.Join(err, errors.New("MCP STDIO application state is unavailable"))
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return errors.New("MCP STDIO socket path is not a socket")
		}
		probe, dialErr := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(ctx, "unix", path)
		if dialErr == nil {
			_ = probe.Close()
			return errors.New("MCP STDIO socket is already in use")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return removeErr
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	listener, err := newMCPStdioListener(ctx, path)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		_ = listener.Close()
		removeMCPStdioSocket(path)
	}()
	go acceptMCPStdio(ctx, listener, adapter, principals)
	return nil
}

func newMCPStdioListener(ctx context.Context, path string) (net.Listener, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	return listener, secureMCPStdioSocket(listener, path)
}

func secureMCPStdioSocket(listener net.Listener, path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return err
	}
	return nil
}

func removeMCPStdioSocket(path string) {
	if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSocket != 0 {
		_ = os.Remove(path)
	}
}

func acceptMCPStdio(ctx context.Context, listener net.Listener, adapter *Gateway, principals PrincipalRepository) {
	connections := make(chan struct{}, mcpStdioConnectionLimit)
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		select {
		case connections <- struct{}{}:
			go func() {
				defer func() { <-connections }()
				serveMCPStdioConnection(ctx, connection, adapter, principals)
			}()
		default:
			_ = connection.SetReadDeadline(time.Now().Add(time.Second))
			_, _ = bufio.NewReaderSize(connection, mcpStdioSelectorLimit+2).ReadSlice('\n')
			_, _ = io.WriteString(connection, "ERR MCP STDIO connection capacity reached\n")
			_ = connection.Close()
		}
	}
}

func serveMCPStdioConnection(ctx context.Context, connection net.Conn, adapter *Gateway, principals PrincipalRepository) {
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReaderSize(connection, mcpStdioSelectorLimit+2)
	line, err := reader.ReadSlice('\n')
	if err != nil || len(line) > mcpStdioSelectorLimit+1 {
		_, _ = io.WriteString(connection, "ERR invalid Owner Profile selector\n")
		return
	}
	profileID := strings.TrimSuffix(string(line), "\n")
	owner, err := mcpStdioOwner(principals, profileID)
	if err != nil {
		_, _ = fmt.Fprintf(connection, "ERR %s\n", err)
		return
	}
	_ = connection.SetReadDeadline(time.Time{})
	if _, err := io.WriteString(connection, "OK\n"); err != nil {
		return
	}
	stdio := adapter.newServer(true, true)
	stdio.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(callContext context.Context, method string, request mcp.Request) (mcp.Result, error) {
			return next(withStdioPrincipal(callContext, owner), method, request)
		}
	})
	_ = stdio.Run(ctx, &mcp.IOTransport{Reader: &mcpSocketReader{Reader: reader, connection: connection}, Writer: connection})
}

func mcpStdioOwner(principals PrincipalRepository, profileID string) (Principal, error) {
	if err := validMCPStdioSelector(profileID); err != nil {
		return Principal{}, err
	}
	owners, err := principals.Owners()
	if err != nil {
		return Principal{}, errors.New("MCP STDIO profile state is unavailable")
	}
	if profileID != "" {
		for _, profile := range owners {
			if profile.ID == profileID && profile.Owner {
				return profile, nil
			}
		}
		return Principal{}, errors.New("MCP STDIO Owner Profile was not found")
	}
	if len(owners) != 1 {
		return Principal{}, errors.New("MCP STDIO requires an Owner Profile ID when multiple Owners exist")
	}
	return owners[0], nil
}

func validMCPStdioSelector(profileID string) error {
	if len(profileID) > mcpStdioSelectorLimit || strings.IndexFunc(profileID, func(character rune) bool {
		return unicode.IsControl(character) || unicode.IsSpace(character)
	}) >= 0 {
		return errors.New("invalid MCP STDIO Owner Profile ID")
	}
	return nil
}

// StdioSocketPath returns the private socket path for one data directory.
func StdioSocketPath(dataDir string) (string, error) {
	if dataDir == "" {
		return "", errors.New("MCP STDIO requires the Server data directory")
	}
	digest := sha256.Sum256([]byte(dataDir))
	return fmt.Sprintf("%s/%s-%x", os.TempDir(), mcpStdioSocketName, digest[:8]), nil
}

type mcpSocketReader struct {
	io.Reader
	connection net.Conn
}

func (reader *mcpSocketReader) Close() error { return reader.connection.Close() }
