package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func tripwirePublic(audit *auditStore, next http.Handler) http.Handler {
	return httpguard.GuardPublicRequests(next, publicInternetRequest, localizedError, localizedNotFound, audit.Tripwire)
}

func publicRequestLimits(next http.Handler) http.Handler {
	return httpguard.LimitPublicRequests(next, publicInternetRequest, localizedError)
}
