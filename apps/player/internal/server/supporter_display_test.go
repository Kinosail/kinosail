package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func supporterDisplayRequest(t *testing.T, handler http.Handler, token, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "https://kinosail.test"+path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestSupporterDisplayAPIAndFormPreserveHonors(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/display", nil), http.StatusOK, `"display":"automatic"`)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LIVING_STANDARD_KEY"}), http.StatusOK)
	before := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String()
	for _, choice := range []string{"hidden", "living-standard", "patron-order", "automatic"} {
		response := supporterDisplayRequest(t, handler, token, http.MethodPut, "/api/v1/supporter/display", "application/json", fmt.Sprintf(`{"display":%q}`, choice))
		assertAPIBody(t, response, http.StatusOK, fmt.Sprintf(`"display":%q`, choice))
		assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/display", nil), http.StatusOK, fmt.Sprintf(`"display":%q`, choice))
	}
	response := supporterDisplayRequest(t, handler, token, http.MethodPost, "/supporter/display", "application/x-www-form-urlencoded", "display=hidden")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/supporter#recognition" {
		t.Fatalf("display form = %d %s", response.Code, response.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/supporter", nil), http.StatusOK, `value="hidden" selected`, "Thank you.", "/static/supporter/badges/living-standard-10.svg", "Your honors")
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/living-standard.svg", nil), http.StatusOK, "Sailcraft — Legacy — Living Standard")
	if after := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String(); before != after {
		t.Fatal("display choice changed signed honors")
	}
	if signer.calls.Load() != 1 {
		t.Fatal("display choice contacted activation service")
	}
}

func TestSupporterDisplayRejectsInvalidInputWithoutSideEffects(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	cases := []struct{ name, method, path, contentType, body string }{
		{"missing", "PUT", "/api/v1/supporter/display", "application/json", `{}`},
		{"null", "PUT", "/api/v1/supporter/display", "application/json", `{"display":null}`},
		{"number", "PUT", "/api/v1/supporter/display", "application/json", `{"display":3}`},
		{"boolean", "PUT", "/api/v1/supporter/display", "application/json", `{"display":true}`},
		{"nested", "PUT", "/api/v1/supporter/display", "application/json", `{"display":{"value":"hidden"}}`},
		{"malformed", "PUT", "/api/v1/supporter/display", "application/json", `{"display":`},
		{"unknown", "PUT", "/api/v1/supporter/display", "application/json", `{"display":"other"}`},
		{"case", "PUT", "/api/v1/supporter/display", "application/json", `{"Display":"hidden"}`},
		{"whitespace", "PUT", "/api/v1/supporter/display", "application/json", `{"display":" hidden"}`},
		{"conflicting", "PUT", "/api/v1/supporter/display", "application/json", `{"display":"hidden","display":"automatic"}`},
		{"extra field", "PUT", "/api/v1/supporter/display", "application/json", `{"display":"hidden","name":"test"}`},
		{"extra document", "PUT", "/api/v1/supporter/display", "application/json", `{"display":"hidden"}{}`},
		{"array", "PUT", "/api/v1/supporter/display", "application/json", `["hidden"]`},
		{"oversized", "PUT", "/api/v1/supporter/display", "application/json", `{"display":"` + strings.Repeat("a", 129) + `"}`},
		{"oversized whitespace", "PUT", "/api/v1/supporter/display", "application/json", `{"display":"hidden"}` + strings.Repeat(" ", 129)},
		{"media type", "PUT", "/api/v1/supporter/display", "text/plain", `{"display":"hidden"}`},
		{"query", "PUT", "/api/v1/supporter/display?other=1", "application/json", `{"display":"hidden"}`},
		{"read query", "GET", "/api/v1/supporter/display?other=1", "", ""},
		{"form missing", "POST", "/supporter/display", "application/x-www-form-urlencoded", ""},
		{"form unknown", "POST", "/supporter/display", "application/x-www-form-urlencoded", "display=other"},
		{"form duplicate", "POST", "/supporter/display", "application/x-www-form-urlencoded", "display=hidden&display=automatic"},
		{"form extra", "POST", "/supporter/display", "application/x-www-form-urlencoded", "display=hidden&extra=1"},
		{"form malformed", "POST", "/supporter/display", "application/x-www-form-urlencoded", "display=%zz"},
		{"form oversized", "POST", "/supporter/display", "application/x-www-form-urlencoded", "display=" + strings.Repeat("a", 129)},
		{"form query", "POST", "/supporter/display?display=hidden", "application/x-www-form-urlencoded", "display=hidden"},
		{"form media type", "POST", "/supporter/display", "text/plain", "display=hidden"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			response := supporterDisplayRequest(t, handler, token, item.method, item.path, item.contentType, item.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid input status=%d body=%s", response.Code, response.Body.String())
			}
			assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/display", nil), http.StatusOK, `"display":"automatic"`)
		})
	}
	if signer.calls.Load() != 0 {
		t.Fatal("invalid display input contacted activation service")
	}
}
