package catalogapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestLibraryProjectsPlayerBrowseContract(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil)
	response := httptest.NewRecorder()
	projected := 0
	Library(func(got *http.Request) (catalog.Result, error) {
		if got != request {
			t.Fatal("browse request was replaced")
		}
		return catalog.Result{Items: []library.Item{{ID: "film"}}, View: "all", Sort: "title", Query: "kino", Letter: "K", Letters: []catalog.Letter{{Label: "K", Count: 1}}, Total: 3, Offset: 1, Limit: 2}, nil
	}, func(got *http.Request, item library.Item) any {
		projected++
		if got != request || item.ID != "film" {
			t.Fatalf("projection = %p %#v", got, item)
		}
		return map[string]string{"id": item.ID}
	})(response, request)
	var result struct {
		Items                []map[string]string
		View, Sort, Query    string
		Letter               string
		Letters              []catalog.Letter
		Total, Offset, Limit int
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil {
		t.Fatalf("response = %d %s; projected = %d", response.Code, response.Body.String(), projected)
	}
	want := struct {
		Items                []map[string]string
		View, Sort, Query    string
		Letter               string
		Letters              []catalog.Letter
		Total, Offset, Limit int
	}{[]map[string]string{{"id": "film"}}, "all", "title", "kino", "K", []catalog.Letter{{Label: "K", Count: 1}}, 3, 1, 2}
	if projected != 1 || !reflect.DeepEqual(result, want) {
		t.Fatalf("projection = %#v; projected = %d", result, projected)
	}
	assertPrivateResponse(t, response)
}

func TestLibraryMapsBrowseErrorsBeforeProjection(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{"invalid", errors.Join(errors.New("bad request"), catalog.ErrInvalidBrowse), http.StatusBadRequest},
		{"unavailable", errors.New("storage unavailable"), http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			projected := false
			Library(func(*http.Request) (catalog.Result, error) { return catalog.Result{}, test.err }, func(*http.Request, library.Item) any {
				projected = true
				return nil
			})(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
			if response.Code != test.status || projected {
				t.Fatalf("response = %d %s; projected = %t", response.Code, response.Body.String(), projected)
			}
			assertPrivateResponse(t, response)
		})
	}
}

func TestLibraryRejectsIncompleteConfiguration(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil)
	for _, handler := range []http.HandlerFunc{
		Library(nil, func(*http.Request, library.Item) any { return nil }),
		Library(func(*http.Request) (catalog.Result, error) { return catalog.Result{}, nil }, nil),
	} {
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusInternalServerError || response.Body.String() != `{"error":"Library API is unavailable"}`+"\n" {
			t.Fatalf("response = %d %s", response.Code, response.Body.String())
		}
	}
}

func assertPrivateResponse(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %#v", response.Header())
	}
}
