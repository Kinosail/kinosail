package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/MikeO7/kinosail/packages/casting"
)

func (service *castService) owned(request *http.Request) (castSession, bool) {
	session, found := service.session(request.PathValue("id"))
	_, allowed := service.viewer(session)
	return session, found && allowed && currentViewer(request).ID == session.profileID && request.URL.RawQuery == "" && !publicInternetRequest(request)
}

func (service *castService) endHTTP(writer http.ResponseWriter, request *http.Request) {
	session, ok := service.owned(request)
	if !ok {
		apiNotFound(writer)
		return
	}
	if body, err := io.ReadAll(io.LimitReader(request.Body, 1)); err != nil || len(body) != 0 {
		apiError(writer, casting.ErrInvalid, http.StatusBadRequest)
		return
	}
	// Revocation must remain possible even when the receiver is offline.
	service.remove(session.ID, session.profileID)
	writer.WriteHeader(http.StatusNoContent)
}

func (service *castService) scanHTTP(writer http.ResponseWriter, request *http.Request) {
	if publicInternetRequest(request) || !currentViewer(request).Permits("stream", true) {
		apiNotFound(writer)
		return
	}
	var input struct{}
	if !readJSON(writer, request, &input) {
		return
	}
	if request.URL.RawQuery != "" {
		apiError(writer, casting.ErrInvalid, http.StatusBadRequest)
		return
	}
	devices, err := service.renderers.Discover(request.Context())
	if err != nil {
		apiError(writer, err, http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, map[string]any{"devices": devices}, http.StatusOK)
}

func (service *castService) configHTTP(writer http.ResponseWriter, request *http.Request) {
	if publicInternetRequest(request) || !currentViewer(request).Permits("stream", true) {
		apiNotFound(writer)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 1))
	if err != nil || len(body) != 0 || request.URL.RawQuery != "" {
		apiError(writer, casting.ErrInvalid, http.StatusBadRequest)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, map[string]string{"appId": service.settings.config.String("integrations.google_cast.app_id")}, http.StatusOK)
}

func (service *castService) statusHTTP(writer http.ResponseWriter, request *http.Request) {
	session, ok := service.owned(request)
	if !ok || session.Protocol != "dlna" {
		apiNotFound(writer)
		return
	}
	status, ok := service.checkedStatus(writer, request, session)
	if !ok {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, status, http.StatusOK)
}

func (service *castService) commandHTTP(writer http.ResponseWriter, request *http.Request) {
	var command casting.Command
	if !readJSON(writer, request, &command) {
		return
	}
	if command.Validate() != nil {
		apiError(writer, casting.ErrInvalid, http.StatusBadRequest)
		return
	}
	session, ok := service.owned(request)
	if !ok || session.Protocol != "dlna" {
		apiNotFound(writer)
		return
	}
	if invalidCastPosition(command.Position, session.Duration) {
		apiError(writer, casting.ErrInvalid, http.StatusBadRequest)
		return
	}
	if _, ok := service.checkedStatus(writer, request, session); !ok {
		return
	}
	if err := service.renderers.Command(request.Context(), session.DeviceID, command); err != nil {
		apiError(writer, err, http.StatusServiceUnavailable)
		return
	}
	if command.Action == "stop" {
		service.remove(session.ID, session.profileID)
	}
	writer.WriteHeader(http.StatusNoContent)
}

func invalidCastPosition(position *float64, duration float64) bool {
	return position != nil && (duration <= 0 || *position > duration)
}

func (service *castService) checkedStatus(writer http.ResponseWriter, request *http.Request, session castSession) (casting.Status, bool) {
	status, err := service.renderers.Status(request.Context(), session.DeviceID)
	if err != nil {
		apiError(writer, err, http.StatusServiceUnavailable)
		return casting.Status{}, false
	}
	if status.Source != service.base+"/cast/"+session.ID+"/media" {
		apiError(writer, errors.New("The TV is playing a different title; connect again"), http.StatusConflict) //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
		return casting.Status{}, false
	}
	return status, true
}
