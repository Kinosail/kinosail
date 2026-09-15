package server

import federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"

const (
	samlConfigurationKey      = federationconfig.SAMLMetadataURLKey
	samlConfigurationGroupKey = federationconfig.SAMLConfigurationKey
	samlConfigurationHTML     = federationconfig.SAMLConfigurationHTML
)

var samlConfigurationKeys = federationconfig.SAMLConfigurationKeys()

type samlConfigurationView = federationconfig.SAMLConfigurationView[settingControl]
