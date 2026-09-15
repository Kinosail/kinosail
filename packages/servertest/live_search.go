package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// LiveSearchReplacesLibraryRegion verifies the shared live-search accessibility contract.
func LiveSearchReplacesLibraryRegion(t *testing.T, handler http.Handler) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	if !strings.Contains(body, `hx-swap="outerHTML"`) {
		t.Fatalf("search does not replace its complete target: %q", body)
	}
	if !strings.Contains(body, `role="status" aria-live="polite" data-library-status`) {
		t.Fatalf("search results are not announced: %q", body)
	}
}
