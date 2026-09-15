package productapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

type metadataService struct {
	item                          library.Item
	found                         bool
	record                        metadata.Record
	saveErr, fetchErr, refreshErr error
	finds, records, saves         int
	fetches, refreshes            int
}

func (service *metadataService) Find(string) (library.Item, bool) {
	service.finds++
	return service.item, service.found
}

func (service *metadataService) Refresh(context.Context) error {
	service.refreshes++
	return service.refreshErr
}

func (service *metadataService) Record(string) metadata.Record {
	service.records++
	return service.record
}

func (service *metadataService) Save(_ string, record metadata.Record) error {
	service.saves++
	service.record = record
	return service.saveErr
}

func (service *metadataService) Fetch(context.Context, library.Item) error {
	service.fetches++
	return service.fetchErr
}

func editRequest(t *testing.T, handler http.Handler, id, body, contentType, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/items/movie"+query, strings.NewReader(body))
	request.SetPathValue("id", id)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func refreshRequest(t *testing.T, handler http.Handler, id, body, contentType, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/movie/refresh"+query, strings.NewReader(body))
	request.SetPathValue("id", id)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestEditMetadataValidatesBeforePersistence(t *testing.T) {
	t.Parallel()
	valid := `{"title":" Title ","year":"2024","plot":"Plot","rating":"PG","tagline":"Tagline","genres":"Drama"}`
	tests := map[string]struct{ id, body, contentType, query string }{
		"missing id":       {body: valid, contentType: "application/json"},
		"oversized id":     {id: strings.Repeat("x", maximumItemID+1), body: valid, contentType: "application/json"},
		"control id":       {id: "bad\nid", body: valid, contentType: "application/json"},
		"query":            {id: "movie", body: valid, contentType: "application/json", query: "?force=true"},
		"missing type":     {id: "movie", body: valid},
		"wrong type":       {id: "movie", body: valid, contentType: "text/plain"},
		"missing body":     {id: "movie", contentType: "application/json"},
		"not object":       {id: "movie", body: `[]`, contentType: "application/json"},
		"malformed":        {id: "movie", body: `{"title":"Movie"`, contentType: "application/json"},
		"unknown":          {id: "movie", body: `{"title":"Movie","other":"x"}`, contentType: "application/json"},
		"wrong field type": {id: "movie", body: `{"title":1}`, contentType: "application/json"},
		"duplicate":        {id: "movie", body: `{"title":"One","title":"Two"}`, contentType: "application/json"},
		"conflicting case": {id: "movie", body: `{"title":"One","Title":"Two"}`, contentType: "application/json"},
		"trailing":         {id: "movie", body: `{"title":"Movie"}{}`, contentType: "application/json"},
		"missing title":    {id: "movie", body: `{}`, contentType: "application/json"},
		"oversized title":  {id: "movie", body: `{"title":"` + strings.Repeat("x", 201) + `"}`, contentType: "application/json"},
		"out of range":     {id: "movie", body: `{"title":"Movie","year":"20x4"}`, contentType: "application/json"},
		"control text":     {id: "movie", body: `{"title":"Movie\u0000"}`, contentType: "application/json"},
		"oversized body":   {id: "movie", body: `{"title":"` + strings.Repeat("x", 1<<20) + `"}`, contentType: "application/json"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			service := &metadataService{item: library.Item{ID: "movie", Kind: "video"}, found: true}
			response := editRequest(t, EditMetadata(service, service), test.id, test.body, test.contentType, test.query)
			if response.Code != http.StatusBadRequest || service.saves != 0 || service.fetches != 0 || service.refreshes != 0 {
				t.Fatalf("response = %d %q, calls = %#v", response.Code, response.Body.String(), service)
			}
		})
	}
}

func TestEditMetadataOutcomeOrdering(t *testing.T) { //nolint:cyclop // The outcome matrix keeps persistence ordering explicit and remains below the repository limit.
	t.Parallel()
	body := `{"title":" Title ","year":"2024","plot":"Plot","rating":"PG","tagline":"Tag","genres":"Drama"}`
	for name, service := range map[string]*metadataService{
		"not found":    {item: library.Item{ID: "movie"}},
		"save failure": {item: library.Item{ID: "movie"}, found: true, saveErr: errors.New("save")},
		"scan failure": {item: library.Item{ID: "movie"}, found: true, refreshErr: errors.New("scan")},
		"success":      {item: library.Item{ID: "movie"}, found: true},
	} {
		t.Run(name, func(t *testing.T) {
			response := editRequest(t, EditMetadata(service, service), "movie", body, "application/json; charset=utf-8", "")
			want := map[string]int{"not found": http.StatusNotFound, "save failure": http.StatusInternalServerError, "scan failure": http.StatusInternalServerError, "success": http.StatusOK}[name]
			if response.Code != want {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
			if name == "save failure" && service.refreshes != 0 {
				t.Fatal("refresh followed failed save")
			}
			if name == "success" && (!service.record.Owner || service.record.Title != "Title" || service.saves != 1 || service.refreshes != 1 || !strings.Contains(response.Body.String(), `"id":"movie"`)) {
				t.Fatalf("success = %#v, %q", service, response.Body.String())
			}
		})
	}
}

func TestEditMetadataAcceptsExactBoundaries(t *testing.T) {
	t.Parallel()
	service := &metadataService{item: library.Item{ID: strings.Repeat("i", maximumItemID), Kind: "video"}, found: true}
	body := `{"title":"` + strings.Repeat("t", 200) + `","year":"2024","plot":"` + strings.Repeat("p", 5000) + `","rating":"` + strings.Repeat("r", 32) + `","tagline":"` + strings.Repeat("a", 300) + `","genres":"` + strings.Repeat("g", 500) + `"}`
	response := editRequest(t, EditMetadata(service, service), service.item.ID, body, "application/json", "")
	if response.Code != http.StatusOK || service.saves != 1 || service.refreshes != 1 {
		t.Fatalf("response = %d %q, calls = %#v", response.Code, response.Body.String(), service)
	}
}

func TestBulkEditMetadataValidatesAndTranslatesOutcomes(t *testing.T) { //nolint:cyclop,gocognit // One bounded matrix proves validation happens before mutation for every transport outcome.
	t.Parallel()
	invalidSelection := errors.New("invalid selection")
	tests := map[string]struct {
		body, content, query string
		apply                func(context.Context, []string, MetadataPatch) (int, error)
		status               int
	}{
		"query":      {body: `{"ids":["one"],"title":"Title"}`, content: "application/json", query: "?force=true", status: http.StatusBadRequest},
		"wrong type": {body: `{"ids":["one"],"title":"Title"}`, content: "text/plain", status: http.StatusBadRequest},
		"malformed":  {body: `{`, content: "application/json", status: http.StatusBadRequest},
		"unknown":    {body: `{"ids":["one"],"unknown":true}`, content: "application/json", status: http.StatusBadRequest},
		"duplicate":  {body: `{"ids":["one"],"title":"A","title":"B"}`, content: "application/json", status: http.StatusBadRequest},
		"invalid": {body: `{"ids":["one"],"title":"Title"}`, content: "application/json", status: http.StatusBadRequest, apply: func(context.Context, []string, MetadataPatch) (int, error) {
			return 0, fmt.Errorf("selection: %w", invalidSelection)
		}},
		"failure": {body: `{"ids":["one"],"title":"Title"}`, content: "application/json", status: http.StatusInternalServerError, apply: func(context.Context, []string, MetadataPatch) (int, error) {
			return 0, errors.New("store failed")
		}},
		"success": {body: `{"ids":["one"],"title":"Title"}`, content: "application/json", status: http.StatusOK, apply: func(_ context.Context, ids []string, patch MetadataPatch) (int, error) {
			if len(ids) != 1 || patch.Title == nil || *patch.Title != "Title" {
				t.Fatalf("input = %#v, %#v", ids, patch)
			}
			return 1, nil
		}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			calls := 0
			handler := BulkEditMetadata(BulkMetadataFunc(func(ctx context.Context, ids []string, patch MetadataPatch) (int, error) {
				calls++
				if test.apply == nil {
					return 0, nil
				}
				return test.apply(ctx, ids, patch)
			}), errors.New("other invalid"), invalidSelection)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/bulk"+test.query, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.content)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || test.status == http.StatusBadRequest && test.apply == nil && calls != 0 {
				t.Fatalf("response = %d %q, calls = %d", response.Code, response.Body.String(), calls)
			}
			if name == "success" && !strings.Contains(response.Body.String(), `"updated":1`) {
				t.Fatalf("success body = %q", response.Body.String())
			}
		})
	}
}

func TestRefreshMetadataValidationAndOrdering(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		service                  *metadataService
		id, body, content, query string
		status                   int
	}{
		"missing id":    {service: &metadataService{found: true}, status: http.StatusBadRequest},
		"oversized id":  {service: &metadataService{found: true}, id: strings.Repeat("x", maximumItemID+1), status: http.StatusBadRequest},
		"query":         {service: &metadataService{found: true}, id: "movie", query: "?force=true", status: http.StatusBadRequest},
		"body":          {service: &metadataService{found: true}, id: "movie", body: "x", status: http.StatusBadRequest},
		"not found":     {service: &metadataService{}, id: "movie", status: http.StatusNotFound},
		"not video":     {service: &metadataService{found: true, item: library.Item{ID: "movie", Kind: "photo"}}, id: "movie", status: http.StatusNotFound},
		"fetch failure": {service: &metadataService{found: true, item: library.Item{ID: "movie", Kind: "video"}, fetchErr: errors.New("fetch")}, id: "movie", status: http.StatusBadGateway},
		"scan failure":  {service: &metadataService{found: true, item: library.Item{ID: "movie", Kind: "video"}, refreshErr: errors.New("scan")}, id: "movie", status: http.StatusBadGateway},
		"success":       {service: &metadataService{found: true, item: library.Item{ID: "movie", Kind: "video"}}, id: "movie", content: "application/x-www-form-urlencoded", status: http.StatusNoContent},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			response := refreshRequest(t, RefreshMetadata(test.service, test.service), test.id, test.body, test.content, test.query)
			if response.Code != test.status {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
			if test.status == http.StatusBadRequest && (test.service.finds != 0 || test.service.fetches != 0 || test.service.refreshes != 0) {
				t.Fatalf("invalid request calls = %#v", test.service)
			}
			if name == "fetch failure" && test.service.refreshes != 0 {
				t.Fatal("refresh followed failed fetch")
			}
		})
	}
}
