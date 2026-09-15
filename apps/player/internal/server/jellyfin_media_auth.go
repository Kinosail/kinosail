package server

import (
	"log/slog"
	"net/http"
)

func (auth *authentication) jellyfinMediaRoutePublic(writer http.ResponseWriter, request *http.Request, matched string, public bool) (bool, bool) {
	if !jellyfinMediaQueryTokenPresent(request) {
		if jellyfinMediaPath(request.URL.Path) && !public {
			slog.WarnContext(request.Context(), "Jellyfin media capability denied before handler", "diagnostic", "[PLAYBACK-AUTH]", "route", matched, "path_shape", jellyfinMediaPathShape(request.URL.Path), "play_session_query", jellyfinQueryShape(request, "playSessionId"), "stream_shape", jellyfinStreamShape(request.URL.Path))
		}
		return public, true
	}
	if jellyfinMediaQueryToken(request) != "" {
		return false, true
	}
	auth.audit.Denied(request, "invalid Jellyfin media token")
	localizedError(writer, request, "authentication required", http.StatusUnauthorized)
	return false, false
}
