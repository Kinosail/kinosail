package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrustedHTTPSFormRejectsAmbiguousRequestsBeforeSave(t *testing.T) {
	t.Parallel()
	valid := "domain=house&token=private&address=192.168.1.2&termsAccepted=true"
	for name, input := range map[string]struct{ target, body, contentType string }{
		"query":          {"/settings/trusted-https?provider=desec", valid, "application/x-www-form-urlencoded"},
		"oversized":      {"/settings/trusted-https", valid + "&token=" + strings.Repeat("x", 16<<10), "application/x-www-form-urlencoded"},
		"wrong encoding": {"/settings/trusted-https", valid, "application/json"},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, input.target, strings.NewReader(input.body))
			request.Header.Set("Content-Type", input.contentType)
			response := httptest.NewRecorder()
			// A nil store proves rejection occurs before the settings operation.
			saveTrustedHTTPS(nil, "/settings")(response, request)
			if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "private") {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}
