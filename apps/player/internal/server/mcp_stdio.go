package server

import (
	"context"
	"io"
	"os"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

func ServeMCPStdio(ctx context.Context, config Config, profileID string, input io.Reader, output io.Writer) error {
	return mcpgateway.ServeStdio(ctx, config.DataDir, profileID, input, output)
}

func OpenMCPStdioRelay(ctx context.Context, config Config, profileID string) (*os.File, error) {
	return mcpgateway.OpenStdioRelay(ctx, config.DataDir, profileID)
}

func startMCPStdioHost(ctx context.Context, dataDir string, adapter *mcpAdapter, profiles *profileStore) error {
	return mcpgateway.StartStdioHost(ctx, dataDir, adapter, mcpPrincipals(profiles))
}
