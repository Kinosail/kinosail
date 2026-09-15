package server

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
)

func authorizedMutation(writer http.ResponseWriter, request *http.Request) (auth.Identity, bool) {
	identity, found := requireOwner(writer, request)
	return identity, found && requireCSRF(writer, request, identity)
}

func actor(identity auth.Identity) string {
	if identity.ViaBearer {
		return "API · " + identity.Name
	}
	return "Web · " + identity.Name
}

func writeMutation(writer http.ResponseWriter, receipt dashboard.Receipt, err error) {
	if err != nil {
		writeDomainError(writer, err)
		return
	}
	writeJSON(writer, map[string]any{"receipt": receipt}, http.StatusOK)
}

func writeDomainError(writer http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, dashboard.ErrNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(err, dashboard.ErrConflict) {
		status = http.StatusConflict
	}
	if errors.Is(err, dashboard.ErrState) {
		status = http.StatusInternalServerError
		err = dashboard.ErrState
	}
	apiError(writer, err, status)
}
