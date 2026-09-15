package server

import (
	"net/http"

	federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"
)

func saveOIDCConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore) {
	federationconfig.SaveOIDCConfiguration(writer, request, settings.changeOIDCConfiguration, localizedError)
}
