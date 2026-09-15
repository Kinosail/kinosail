package server

import (
	"net/http"

	scimapp "github.com/MikeO7/kinosail/packages/scim/app"
)

func saveSCIMConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore) {
	scimapp.SaveConfiguration(writer, request, settings.changeSCIMConfiguration, localizedError)
}
