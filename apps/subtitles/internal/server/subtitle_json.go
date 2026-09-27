package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func decodeSubtitleValue(raw json.RawMessage, target any) bool {
	return !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && json.Unmarshal(raw, target) == nil
}

func readSubtitleJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	if httpguard.DecodeUniqueJSON(request.Body, 4096, target) != nil {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	return true
}
