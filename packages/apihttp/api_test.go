package apihttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestReadJSONAndResponses(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"name":"Player"}`))
	response := httptest.NewRecorder()
	var input struct{ Name string }
	if !ReadJSON(response, request, &input) || input.Name != "Player" {
		t.Fatalf("valid JSON = %#v, %s", input, response.Body.String())
	}

	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"unknown":true}`))
	response = httptest.NewRecorder()
	if ReadJSON(response, request, &input) || response.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON = %d %s", response.Code, response.Body.String())
	}
	assertPrivateJSON(t, response)

	response = httptest.NewRecorder()
	NotFound(response)
	if response.Code != http.StatusNotFound || response.Body.String() != "{\"error\":\"not found\"}\n" {
		t.Fatalf("not found = %d %s", response.Code, response.Body.String())
	}
	assertPrivateJSON(t, response)
}

func assertPrivateJSON(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response headers = %#v", response.Header())
	}
	var value map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil || value["error"] == "" {
		t.Fatalf("response body = %q, %v", response.Body.String(), err)
	}
}

func TestStoreStatus(t *testing.T) {
	t.Parallel()
	invalid := errors.New("invalid state")
	for name, test := range map[string]struct {
		err  error
		want int
	}{
		"invalid": {fmtError(invalid), http.StatusBadRequest},
		"missing": {fmtError(os.ErrNotExist), http.StatusNotFound},
		"other":   {errors.New("failed"), http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			if got := StoreStatus(test.err, invalid); got != test.want {
				t.Fatalf("StoreStatus = %d, want %d", got, test.want)
			}
		})
	}
	if got := StoreStatus(errors.Join(errors.New("wrapped"), errors.New("file does not exist")), invalid); got != http.StatusInternalServerError {
		t.Fatalf("unrelated status = %d", got)
	}
	if got := StoreStatus(nil, nil); got != http.StatusInternalServerError {
		t.Fatalf("nil status contract = %d", got)
	}
}

func fmtError(err error) error { return errors.Join(errors.New("operation failed"), err) }

func TestRouting(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/items/{id}", func(writer http.ResponseWriter, request *http.Request) {
		WriteJSON(writer, map[string]string{"id": request.PathValue("id")}, http.StatusOK)
	})
	mux.HandleFunc("DELETE /api/v1/items/{id}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	handler := Routing(mux)

	response := request(t, handler, http.MethodGet, "/api/v1/items/movie")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"movie"`) {
		t.Fatalf("registered route = %d %s", response.Code, response.Body.String())
	}
	response = request(t, handler, http.MethodPost, "/api/v1/items/movie")
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD, DELETE" {
		t.Fatalf("method response = %d %#v %s", response.Code, response.Header(), response.Body.String())
	}
	response = request(t, handler, http.MethodGet, "/api/v1/missing")
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing API route = %d %s", response.Code, response.Body.String())
	}
	response = request(t, handler, http.MethodGet, "/healthz")
	if response.Code != http.StatusOK {
		t.Fatalf("non-API route = %d", response.Code)
	}
	allowed := methods(mux, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/movie", nil))
	if strings.Join(allowed, ",") != "GET,HEAD,DELETE" {
		t.Fatalf("methods = %#v", allowed)
	}
}

func request(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return response
}
