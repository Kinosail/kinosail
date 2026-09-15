package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/scim"
	scimapp "github.com/MikeO7/kinosail/packages/scim/app"
)

func scimRepository(profiles *profileStore) scim.Repository {
	return scimapp.NewRepository(profiles.scimProfiles, profiles.scimProfile, profiles.createSCIMProfile, profiles.updateSCIMProfile, profiles.deleteSCIMProfile, scimProtocolProfile)
}

func registerSCIM(mux *http.ServeMux, config SCIMConfig, profiles *profileStore) {
	scim.Register(mux, config, scimRepository(profiles))
}
