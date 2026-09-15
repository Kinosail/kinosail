// Package apihttp owns Player's common JSON API transport behavior.
package apihttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// ReadJSON decodes one strict bounded request or writes a bad-request response.
func ReadJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	if err := httpguard.DecodeRequestJSON(writer, request, target); err != nil {
		Error(writer, err, http.StatusBadRequest)
		return false
	}
	return true
}

// WriteJSON writes one private JSON response.
func WriteJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

// Error writes one stable JSON error response.
func Error(writer http.ResponseWriter, err error, status int) {
	WriteJSON(writer, map[string]string{"error": err.Error()}, status)
}

// NotFound writes Player's API not-found response.
func NotFound(writer http.ResponseWriter) {
	Error(writer, errors.New("not found"), http.StatusNotFound)
}

// StoreStatus maps one persisted-state error to its API status.
func StoreStatus(err, invalid error) int {
	if invalid != nil && errors.Is(err, invalid) {
		return http.StatusBadRequest
	}
	if errors.Is(err, os.ErrNotExist) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// Routing adds versioned API method and not-found responses around one mux.
func Routing(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/v1") {
			if _, pattern := mux.Handler(request); pattern == "" {
				allowed := methods(mux, request)
				if len(allowed) != 0 {
					writer.Header().Set("Allow", strings.Join(allowed, ", "))
					Error(writer, errors.New("method not allowed"), http.StatusMethodNotAllowed)
					return
				}
				NotFound(writer)
				return
			}
		}
		mux.ServeHTTP(writer, request)
	})
}

func methods(mux *http.ServeMux, request *http.Request) []string {
	allowed := make([]string, 0, 6)
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		probe := request.Clone(request.Context())
		probe.Method = method
		if _, pattern := mux.Handler(probe); pattern != "" {
			allowed = append(allowed, method)
		}
	}
	return allowed
}
