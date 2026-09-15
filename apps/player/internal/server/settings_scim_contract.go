package server

import scimapp "github.com/MikeO7/kinosail/packages/scim/app"

const (
	scimConfigurationKey  = scimapp.ConfigurationKey
	scimConfigurationHTML = scimapp.ConfigurationHTML
)

var (
	scimConfigurationKeys = scimapp.ConfigurationKeys()
	scimExpired           = scimapp.Expired
	scimEndpoint          = scimapp.Endpoint
)

type scimConfigurationView = scimapp.ConfigurationView[settingControl]
