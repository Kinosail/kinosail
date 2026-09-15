package server

import (
	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"
)

func samlConfigured(config configuration.Snapshot) bool {
	return federationconfig.SAMLConfigured(config.String)
}
