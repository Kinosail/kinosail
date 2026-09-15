package scim

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

func writeSCIMValidationError(writer http.ResponseWriter, err error) {
	var validation *scimValidationError
	var noTarget *scimNoTargetError
	if errors.As(err, &noTarget) {
		writeSCIMError(writer, http.StatusBadRequest, "noTarget", noTarget.detail)
		return
	}
	if errors.As(err, &validation) {
		writeSCIMError(writer, http.StatusBadRequest, "invalidValue", validation.detail)
		return
	}
	writeSCIMError(writer, http.StatusBadRequest, "invalidSyntax", "the SCIM request is invalid")
}

func writeSCIMStoreError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrProfileNotFound):
		writeSCIMError(writer, http.StatusNotFound, "", err.Error())
	case errors.Is(err, ErrConflict):
		writeSCIMError(writer, http.StatusConflict, "uniqueness", err.Error())
	case errors.Is(err, ErrPrecondition):
		writeSCIMError(writer, http.StatusPreconditionFailed, "", "If-Match does not match the current resource")
	case errors.Is(err, ErrSetupRequired):
		writeSCIMError(writer, http.StatusServiceUnavailable, "", err.Error())
	default:
		writeSCIMError(writer, http.StatusInternalServerError, "", "SCIM resource could not be persisted")
	}
}

func writeSCIMError(writer http.ResponseWriter, status int, scimType, detail string) {
	result := map[string]any{"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:Error"}, "status": strconv.Itoa(status), "detail": detail}
	if scimType != "" {
		result["scimType"] = scimType
	}
	writeSCIMJSON(writer, result, status)
}

func writeSCIMJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/scim+json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
