package server

import (
	"net/http"

	federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"
)

func saveSAMLConfiguration(writer http.ResponseWriter, request *http.Request, store *settingsStore) {
	federationconfig.SaveSAMLConfiguration(writer, request, store.changeSAMLConfiguration, localizedError)
}
