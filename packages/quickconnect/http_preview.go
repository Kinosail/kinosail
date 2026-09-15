package quickconnect

import (
	"net/http"
	"net/url"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

// PreviewAPI exposes a read-only approval preview to signed-in clients.
func (application *Application) PreviewAPI(writer http.ResponseWriter, request *http.Request) {
	if !application.allowApprovalRead(writer, request) {
		return
	}
	pending, err := application.broker.Preview(request.PathValue("code"), application.viewer(request))
	if err != nil {
		apihttp.Error(writer, err, http.StatusBadRequest)
		return
	}
	apihttp.WriteJSON(writer, pending, http.StatusOK)
}

// PendingTVsAPI exposes only bounded pending TV previews, never polling secrets.
func (application *Application) PendingTVsAPI(writer http.ResponseWriter, request *http.Request) {
	if !application.allowApprovalRead(writer, request) {
		return
	}
	pending, err := application.broker.PendingTVs(application.viewer(request))
	if err != nil {
		apihttp.Error(writer, err, http.StatusBadRequest)
		return
	}
	apihttp.WriteJSON(writer, pending, http.StatusOK)
}

func (application *Application) viewer(request *http.Request) Viewer {
	profile := application.adapter.Current(request)
	return Viewer{ID: profile.ID}
}

func (application *Application) allowApprovalRead(writer http.ResponseWriter, request *http.Request) bool {
	writer.Header().Set("Cache-Control", "no-store")
	if identitycore.RemoteRequest(request) {
		apihttp.NotFound(writer)
		return false
	}
	if request.URL.ForceQuery || !httpguard.EmptyMutationRequest(writer, request) {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return false
	}
	if !application.limiters.Requests.Allow("approval:"+httpguard.RemoteHost(request.RemoteAddr), 60) {
		tooMany(writer, "too many Quick Connect requests")
		return false
	}
	return true
}

// ReadLinkCode strictly parses a six-digit code from a scanned approval link.
func ReadLinkCode(request *http.Request) (string, bool) {
	if len(request.URL.RawQuery) > 64 || request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
		return "", false
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil || len(query) != 1 || len(query["code"]) != 1 || !numericCode(query.Get("code")) {
		return "", false
	}
	return query.Get("code"), true
}
