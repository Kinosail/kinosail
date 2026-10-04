package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

type subtitleActivationKey struct{}
type subtitleActivation struct{ id, digest string }

func (manager *subtitleManager) registerSubtitleOperations(mux *http.ServeMux, auth *authentication) {
	mux.Handle("POST /api/v1/subtitle-operations", auth.owner(http.HandlerFunc(manager.prepareSubtitleOperationAPI)))
	mux.Handle("GET /api/v1/subtitle-operations/{operation}", auth.owner(http.HandlerFunc(manager.subtitleOperationAPI)))
	mux.Handle("GET /api/v1/subtitle-operations/{operation}/result", auth.owner(http.HandlerFunc(manager.subtitleOperationResultAPI)))
}

func (manager *subtitleManager) prepareSubtitleOperationAPI(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	var input struct {
		Action string  `json:"action"`
		Item   *string `json:"item"`
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || request.URL.RawQuery != "" || httpguard.DecodeUniqueJSON(request.Body, 1024, &input) != nil {
		manager.operations.rejected(writer, request, "prepare", http.StatusBadRequest)
		return
	}
	item := ""
	if input.Item != nil {
		item = *input.Item
	}
	batch := oneOf(input.Action, "maintain", "fetch-wanted")
	if !validSubtitleOperationInput(input.Action, item) || batch && input.Item != nil {
		manager.operations.rejected(writer, request, "prepare", http.StatusBadRequest)
		return
	}
	if !batch {
		if _, status, err := manager.subtitleItem(request, item); err != nil {
			manager.operations.rejected(writer, request, input.Action, status)
			return
		}
	}
	receipt, status := manager.operations.prepare(request, input.Action, item)
	if status != http.StatusCreated {
		manager.operations.rejected(writer, request, input.Action, status)
		return
	}
	writer.Header().Set("Location", "/api/v1/subtitle-operations/"+receipt.ID)
	writeSubtitleOperationReceipt(writer, receipt, status)
}

func (manager *subtitleManager) subtitleOperationAPI(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	if request.URL.RawQuery != "" {
		manager.operations.rejected(writer, request, "status", http.StatusBadRequest)
		return
	}
	receipt, status := manager.operations.read(request, request.PathValue("operation"))
	if status != http.StatusOK {
		manager.operations.rejected(writer, request, "status", status)
		return
	}
	writeSubtitleOperationReceipt(writer, receipt, status)
}

func (manager *subtitleManager) subtitleOperationResultAPI(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	if request.URL.RawQuery != "" {
		manager.operations.rejected(writer, request, "result", http.StatusBadRequest)
		return
	}
	data, status := manager.operations.result(request, request.PathValue("operation"))
	if status != http.StatusOK {
		manager.operations.rejected(writer, request, "result", status)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(status)
	_, _ = writer.Write(data)
}

func writeSubtitleOperationReceipt(writer http.ResponseWriter, receipt subtitleOperationReceipt, status int) {
	writeSubtitleOperationJSON(writer, struct {
		subtitleOperationReceipt
		URL string `json:"statusURL"`
	}{receipt, "/api/v1/subtitle-operations/" + receipt.ID}, status)
}

func writeSubtitleOperationJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func (manager *subtitleManager) mutation(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		values, supplied := request.Header[http.CanonicalHeaderKey("X-Kinosail-Operation")]
		if !supplied {
			manager.operations.admission.legacyHandler(next)(writer, request)
			return
		}
		if len(values) != 1 || !validSubtitleFingerprint(values[0]) {
			manager.operations.rejected(writer, request, action, http.StatusBadRequest)
			return
		}
		manager.operations.mu.Lock()
		record, status := manager.operations.record(request, values[0])
		manager.operations.mu.Unlock()
		if status == http.StatusOK && (record.Action != action || record.Item != request.PathValue("id")) {
			status = http.StatusConflict
		}
		if status != http.StatusOK {
			manager.operations.rejected(writer, request, action, status)
			return
		}
		if record.Item != "" {
			if _, status, err := manager.subtitleItem(request, record.Item); err != nil {
				manager.operations.rejected(writer, request, action, status)
				return
			}
		}
		data, err := subtitleOperationBody(request.Body)
		if err != nil {
			apiError(writer, errors.New("subtitle operation body is invalid"), http.StatusBadRequest)
			return
		}
		digest := sha256.Sum256(data)
		activation := subtitleActivation{values[0], hex.EncodeToString(digest[:])}
		request = request.WithContext(context.WithValue(request.Context(), subtitleActivationKey{}, activation))
		request.Body = io.NopCloser(bytes.NewReader(data))
		next(writer, request)
	}
}

// Typed adapters call this after their existing request validation. Invalid
// bodies leave preparation untouched; jobs reuse the same application work.
func (manager *subtitleManager) runPrepared(writer http.ResponseWriter, request *http.Request, work http.HandlerFunc) bool {
	activation, found := request.Context().Value(subtitleActivationKey{}).(subtitleActivation)
	if !found {
		return false
	}
	receipt, status := manager.operations.activate(request, activation, work)
	if status != http.StatusAccepted {
		manager.operations.rejected(writer, request, receipt.Action, status)
		return true
	}
	writeSubtitleOperationReceipt(writer, receipt, status)
	return true
}
