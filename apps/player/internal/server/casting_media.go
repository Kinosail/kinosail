package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

func (service *castService) mediaHTTP(writer http.ResponseWriter, request *http.Request) {
	session, found := service.session(request.PathValue("id"))
	token, valid := castMediaTicket(request, session)
	if !found || !valid {
		http.NotFound(writer, request)
		return
	}
	viewer, allowed := service.viewer(session)
	if !allowed {
		http.NotFound(writer, request)
		return
	}
	viewerContext := context.WithValue(request.Context(), viewerContextKey{}, viewer)
	authorized := request.Clone(viewerContext)
	authorized.URL.RawQuery = ""
	authorized.SetPathValue("id", session.itemID)
	item, found := visibleItem(authorized, service.index, session.itemID)
	if !castSourceCurrent(item.Path, session.version, found) {
		http.NotFound(writer, request)
		return
	}
	isSubtitle := strings.Contains(request.URL.Path, "/subtitles/")
	isHLS := strings.Contains(request.URL.Path, "/hls/")
	if !validCastMediaRoute(request, session, isSubtitle, isHLS) {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	writer.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type")
	writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if isSubtitle {
		service.serveCastSubtitle(writer, authorized, session)
		return
	}
	if isHLS {
		authorized = authorized.WithContext(context.WithValue(viewerContext, castTicketKey{}, token))
		service.hls.serveRecipe(writer, authorized, item, recipeFor(session.plan), request.PathValue("file"))
		return
	}
	serveFile(service.index, playback.MediaPath, session.ContentType)(writer, authorized)
}

// The track number was validated against this session before response headers.
func (service *castService) serveCastSubtitle(writer http.ResponseWriter, request *http.Request, session castSession) {
	track, _ := strconv.Atoi(request.PathValue("track"))
	parts := strings.Split(session.Tracks[track-1].source, "/")
	switch {
	case len(parts) == 5 && parts[3] == "embedded":
		request.SetPathValue("stream", parts[4])
		service.probe.serveEmbedded(service.index)(writer, request)
	case len(parts) == 4:
		request.SetPathValue("track", parts[3])
		serveSubtitle(service.index)(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func castMediaTicket(request *http.Request, session castSession) (string, bool) {
	query, err := url.ParseQuery(request.URL.RawQuery)
	token := query.Get("ticket")
	proof := sha256.Sum256([]byte(token))
	valid := err == nil && len(query) == 1 && len(query["ticket"]) == 1 && len(token) == 64 && subtle.ConstantTimeCompare(proof[:], session.proof[:]) == 1 && !publicInternetRequest(request)
	return token, valid
}

func validCastMediaRoute(request *http.Request, session castSession, subtitle, hls bool) bool {
	if subtitle {
		track, err := strconv.Atoi(request.PathValue("track"))
		return err == nil && strconv.Itoa(track) == request.PathValue("track") && track >= 1 && track <= len(session.Tracks)
	}
	return hls == (session.plan.Mode != "direct") && (!hls || hlsFile(request.PathValue("file")))
}

func castSourceCurrent(path, version string, found bool) bool {
	return found && sourceVersion(path) == version
}
