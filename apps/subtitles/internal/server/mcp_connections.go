package server

import (
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

type mcpConnections struct {
	*mcpgateway.Connections
	profiles *profileStore
	dataDir  string
}

func newMCPConnections(issuer, dataDir string, profiles *profileStore, stateDB *database.Store) *mcpConnections {
	store := mcpgateway.ApplicationState(dataDir, func(path string, target any) (bool, error) {
		return loadState(stateDB, path, target)
	}, statePersistence(stateDB))
	connections := mcpgateway.NewConnections(mcpgateway.ConnectionConfig{
		Issuer: issuer, Principals: mcpPrincipals(profiles), Store: store,
		Approval: mcpApproval, Error: mcpError, SessionKey: sessionKey,
	})
	return &mcpConnections{Connections: connections, profiles: profiles, dataDir: dataDir}
}
