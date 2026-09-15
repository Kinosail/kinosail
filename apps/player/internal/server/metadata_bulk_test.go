package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBulkMetadataEditUsesAPIAndWebAdapters(t *testing.T) {
	t.Parallel()
	handler, token := apiServer(t)
	ids := apiVideoIDs(t, handler, token)

	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/metadata/bulk", map[string]any{
		"ids": ids, "genres": "Drama",
	}), http.StatusOK, `"updated":2`)
	page := apiCall(t, handler, token, http.MethodGet, "/metadata/bulk", nil)
	assertAPIBody(t, page, http.StatusOK, "Edit multiple items", ids[0], ids[1])

	response := webFormCall(t, handler, token, "/metadata/bulk", url.Values{
		"itemIds": ids, "tagline": {"Shared tagline"},
	})
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("web bulk edit = %d %q", response.Code, response.Body.String())
	}
	for _, id := range ids {
		assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/items/"+id, nil), http.StatusOK, `"tagline":"Shared tagline"`, `"genres":"Drama"`)
	}
}

func TestBulkMetadataEditRejectsInvalidSelectionWithoutSideEffects(t *testing.T) {
	t.Parallel()
	handler, token := apiServer(t)
	ids := apiVideoIDs(t, handler, token)
	before := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil).Body.String()

	for name, input := range map[string]any{
		"duplicate selection": map[string]any{"ids": []string{ids[0], ids[0]}, "genres": "Drama"},
		"missing item":        map[string]any{"ids": []string{ids[0], "0000000000000000"}, "genres": "Drama"},
		"oversized title":     map[string]any{"ids": ids, "title": strings.Repeat("x", 201)},
		"unknown field":       map[string]any{"ids": ids, "network": "public"},
	} {
		t.Run(name, func(t *testing.T) {
			assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/metadata/bulk", input), http.StatusBadRequest)
			after := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil).Body.String()
			if after != before {
				t.Fatalf("invalid bulk edit changed library metadata: before=%q after=%q", before, after)
			}
		})
	}
}

func TestMetadataWebEditRejectsAmbiguousInputWithoutSideEffects(t *testing.T) {
	t.Parallel()
	handler, token := apiServer(t)
	id := apiVideoIDs(t, handler, token)[0]
	before := apiCall(t, handler, token, http.MethodGet, "/api/v1/items/"+id, nil).Body.String()
	for name, input := range map[string]struct{ path, body, contentType string }{
		"wrong content type": {"/metadata/" + id, "title=Changed", "text/plain"},
		"query":              {"/metadata/" + id + "?extra=true", "title=Changed", "application/x-www-form-urlencoded"},
		"unknown field":      {"/metadata/" + id, "title=Changed&extra=true", "application/x-www-form-urlencoded"},
		"duplicate":          {"/metadata/" + id, "title=Changed&title=Again", "application/x-www-form-urlencoded"},
		"malformed year":     {"/metadata/" + id, "title=Changed&year=abcd", "application/x-www-form-urlencoded"},
		"oversized":          {"/metadata/" + id, "title=" + strings.Repeat("x", 9<<10), "application/x-www-form-urlencoded"},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, input.path, strings.NewReader(input.body))
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("Content-Type", input.contentType)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid metadata edit = %d %q", response.Code, response.Body.String())
			}
			after := apiCall(t, handler, token, http.MethodGet, "/api/v1/items/"+id, nil).Body.String()
			if after != before {
				t.Fatalf("invalid metadata edit changed item: before=%q after=%q", before, after)
			}
		})
	}
}

func apiVideoIDs(t *testing.T, handler http.Handler, token string) []string {
	t.Helper()
	response := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"items"`
	}
	mustJSON(t, response, &catalog)
	ids := make([]string, 0, 2)
	for _, item := range catalog.Items {
		if item.Kind == "video" {
			ids = append(ids, item.ID)
		}
		if len(ids) == 2 {
			return ids
		}
	}
	t.Fatalf("video fixture IDs = %v", ids)
	return nil
}
