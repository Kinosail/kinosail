package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

var remoteReadinessView = newLocalizedTemplate("remote-readiness", remoteaccess.ReadinessViewSource("Player", "75"))

type remoteReadiness = remoteaccess.Readiness

var securePublicReadiness = remoteaccess.SecurePublicReadiness

func showRemoteReadiness(internet *remoteaccess.Manager, profiles *profileStore) http.HandlerFunc {
	return remoteaccess.NewReadinessHandler(internet, func() ([]viewerProfile, error) {
		return profiles.list(), profiles.err
	}, remoteReadinessView, localizedError)
}
