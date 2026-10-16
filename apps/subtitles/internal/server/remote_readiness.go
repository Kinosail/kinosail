package server

import (
	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

var remoteReadinessView = newLocalizedTemplate("remote-readiness", remoteaccess.ReadinessViewSource("Subtitles", "62"))

type remoteReadiness = remoteaccess.Readiness

var securePublicReadiness = remoteaccess.SecurePublicReadiness
