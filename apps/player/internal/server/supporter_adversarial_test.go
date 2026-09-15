package server_test

import (
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupporterRejectsFreshV2CertificateWithoutChangingState(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.version = 2
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LEGACY_CERTIFICATE_KEY"}), http.StatusBadRequest, "invalid certificate")
	status := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil)
	assertAPIBody(t, status, http.StatusOK, `"tier":"free"`, `"active":false`)
	if strings.Contains(status.Body.String(), "livingStandard") || strings.Contains(status.Body.String(), "patronOrder") {
		t.Fatalf("fresh v2 certificate changed local state: %q", status.Body.String())
	}
}

func TestSupporterEscapesPublicRecognitionNameInHTMLAndSVG(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	name := `Captain <script>alert("fleet")</script> & First Mate`
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "SAFE_PUBLIC_NAME_KEY", "recognitionName": name}), http.StatusOK, `"tier":"legacy"`)
	for _, response := range []*httptest.ResponseRecorder{
		apiCall(t, handler, token, http.MethodGet, "/supporter", nil),
		apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/living-standard.svg", nil),
	} {
		body := response.Body.String()
		if response.Code != http.StatusOK || strings.Contains(body, name) || strings.Contains(body, "<script>") || !strings.Contains(body, html.EscapeString(name)) {
			t.Fatalf("unsafe public recognition rendering = %d %q", response.Code, body)
		}
	}
}
