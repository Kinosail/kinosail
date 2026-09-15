package viewing

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type httpHandlersStub struct{ calls int }

func (stub *httpHandlersStub) handle(writer http.ResponseWriter) {
	stub.calls++
	writer.WriteHeader(http.StatusNoContent)
}

func (stub *httpHandlersStub) APIPreview(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) APIApply(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) APIListSyncs(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) APICreateSync(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) APIRunSync(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) APIDeleteSync(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) WebPreview(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) WebApply(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) WebOnboarding(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) WebCreateSync(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) WebRunSync(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func (stub *httpHandlersStub) WebDeleteSync(writer http.ResponseWriter, _ *http.Request) {
	stub.handle(writer)
}

func TestRegisterHTTPInstallsCanonicalRoutes(t *testing.T) {
	mux, handlers, owners := http.NewServeMux(), &httpHandlersStub{}, 0
	RegisterHTTP(mux, func(handler http.Handler) http.Handler { owners++; return handler }, handlers)
	requests := [][2]string{
		{http.MethodPost, "/api/v1/viewing-imports/preview"},
		{http.MethodPost, "/api/v1/viewing-imports/ABCDEFGHIJKLMNOP234567/apply"},
		{http.MethodGet, "/api/v1/viewing-syncs"},
		{http.MethodPost, "/api/v1/viewing-syncs"},
		{http.MethodPost, "/api/v1/viewing-syncs/ABCDEFGHIJKLMNOP234567/run"},
		{http.MethodDelete, "/api/v1/viewing-syncs/ABCDEFGHIJKLMNOP234567"},
		{http.MethodPost, "/settings/viewing-imports/preview"},
		{http.MethodPost, "/settings/viewing-imports/apply"},
		{http.MethodGet, "/onboarding/migrate"},
		{http.MethodPost, "/onboarding/viewing-imports/preview"},
		{http.MethodPost, "/onboarding/viewing-imports/apply"},
		{http.MethodPost, "/settings/viewing-syncs"},
		{http.MethodPost, "/settings/viewing-syncs/run"},
		{http.MethodPost, "/settings/viewing-syncs/remove"},
	}
	for _, route := range requests {
		response := &responseWriterStub{header: make(http.Header)}
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), route[0], route[1], nil))
		if response.status != http.StatusNoContent {
			t.Fatalf("%s %s = %d", route[0], route[1], response.status)
		}
	}
	if handlers.calls != len(requests) || owners != len(requests) {
		t.Fatalf("calls=%d owners=%d", handlers.calls, owners)
	}
}

func formRequest(body string) *http.Request {
	return &http.Request{Method: http.MethodPost, Header: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}, URL: &url.URL{}, Body: io.NopCloser(strings.NewReader(body))}
}

type responseWriterStub struct {
	header http.Header
	status int
}

func (writer *responseWriterStub) Header() http.Header        { return writer.header }
func (writer *responseWriterStub) Write([]byte) (int, error)  { return 0, nil }
func (writer *responseWriterStub) WriteHeader(statusCode int) { writer.status = statusCode }
