//go:build !windows

package mcpgateway

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

type StdioCommandExec func(string, []string, []string) error

func RunStdioCommand(relayPath string, serve func() error, openRelay func() (*os.File, error), execute StdioCommandExec) error { //nolint:cyclop // Each relay resource boundary fails closed in one process handoff.
	if relayPath == "" || len(relayPath) > 4096 || !filepath.IsAbs(relayPath) || filepath.Clean(relayPath) != relayPath || serve == nil || openRelay == nil || execute == nil {
		return errors.New("invalid MCP STDIO relay configuration")
	}
	if _, err := os.Stat(relayPath); err != nil {
		return serve()
	}
	relay, err := openRelay()
	if err != nil {
		return err
	}
	if relay == nil {
		return errors.New("MCP STDIO relay returned no socket")
	}
	defer relay.Close()
	if _, err := unix.FcntlInt(relay.Fd(), unix.F_SETFD, 0); err != nil {
		return err
	}
	return execute(relayPath, []string{"socat", "STDIO", "FD:" + strconv.FormatUint(uint64(relay.Fd()), 10)}, os.Environ())
}
