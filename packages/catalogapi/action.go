package catalogapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpcompress"
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

func (action apiAction) serveListing(writer http.ResponseWriter, request *http.Request) {
	var body bytes.Buffer
	if action.err != nil || action.notFound || action.status != http.StatusOK || json.NewEncoder(&body).Encode(action.body) != nil {
		action.serve(writer)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpcompress.Write(writer, request, "application/json", body.Bytes())
}
