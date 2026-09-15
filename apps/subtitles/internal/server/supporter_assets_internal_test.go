package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupporterCombinedStylesheetUsesFinalImmutableAsset(t *testing.T) {
	handler := New(Config{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css?v=impeccable-1", nil))
	expected := append(append(append([]byte(nil), appCSS...), supporterCSS...), subtitleDashboardCSS...)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || !bytes.Equal(response.Body.Bytes(), expected) {
		t.Fatalf("combined stylesheet = %d, cache %q, bytes %d; want %d", response.Code, response.Header().Get("Cache-Control"), response.Body.Len(), len(expected))
	}
}
