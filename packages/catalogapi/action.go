package catalogapi

import (
	"encoding/json"
	"errors"
	"net/http"
)

type apiAction struct {
	body     any
	err      error
	status   int
	notFound bool
}

func (action apiAction) serve(writer http.ResponseWriter) {
	if action.notFound {
		action.err, action.status = errors.New("not found"), http.StatusNotFound
	}
	if action.err != nil {
		action.body = map[string]string{"error": action.err.Error()}
	}
	if action.status == http.StatusNoContent {
		writer.WriteHeader(action.status)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(action.status)
	_ = json.NewEncoder(writer).Encode(action.body)
}
