package apihttp

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type registrationFixture struct{ calls []string }

func (fixture *registrationFixture) record(name string) { fixture.calls = append(fixture.calls, name) }

func (fixture *registrationFixture) RegisterFoundationAPI(mux *http.ServeMux) {
	fixture.record("foundation")
	mux.HandleFunc("GET /api/v1/library", func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) })
}

func (fixture *registrationFixture) RegisterDownloadsAPI(*http.ServeMux) {
	fixture.record("downloads")
}

func (fixture *registrationFixture) RegisterAdminAPI(*http.ServeMux) {
	fixture.record("admin")
}

func (fixture *registrationFixture) RegisterSupporterAPI(*http.ServeMux) {
	fixture.record("supporter")
}

func (fixture *registrationFixture) RegisterProductAPI(*http.ServeMux) {
	fixture.record("product")
}

func TestRegisterPreservesPlayerAPIRouteOrder(t *testing.T) {
	fixture, mux := &registrationFixture{}, http.NewServeMux()
	Register(mux, fixture)
	want := []string{"foundation", "downloads", "admin", "supporter", "product"}
	if !reflect.DeepEqual(fixture.calls, want) {
		t.Fatalf("registration order = %#v, want %#v", fixture.calls, want)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("Library status = %d", response.Code)
	}
}
