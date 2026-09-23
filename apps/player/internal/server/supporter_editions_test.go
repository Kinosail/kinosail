package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func activateThreeEditions(t *testing.T, handler http.Handler, token string, signer *supporterSigner) {
	t.Helper()
	signer.version = 5
	for _, edition := range []string{"one-time", "monthly", "yearly"} {
		signer.edition = edition
		signer.sustaining = edition != "one-time"
		signer.expiresAt = nil
		if signer.sustaining {
			signer.expiresAt = signer.now.Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
		}
		assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "SUPPORTER_" + edition}), http.StatusOK)
	}
}

func TestThreeEditionCollectionAndVisibility(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	activateThreeEditions(t, handler, token, signer)
	collection := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/collection", nil)
	assertAPIBody(t, collection, http.StatusOK, `"edition":"one-time"`, `"edition":"monthly"`, `"edition":"yearly"`)
	for _, secret := range []string{"supporterId", "installationKey", "recognitionName", "publicKey", "signature", "certificate", "SUPPORTER_"} {
		if strings.Contains(collection.Body.String(), secret) {
			t.Fatal("collection exposed", secret)
		}
	}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/supporter/collection", nil), http.StatusUnauthorized)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/collection?edition=monthly", nil), http.StatusBadRequest)
	assertAPIBody(t, supporterDisplayRequest(t, handler, token, "PUT", "/api/v1/supporter/display", "application/json", `{"display":"hidden"}`), http.StatusOK)
	assertAPIBody(t, apiCall(t, handler, token, "GET", "/api/v1/supporter/collection", nil), http.StatusOK, `"display":"hidden"`, `"edition":"monthly"`)
	assertAPIBody(t, apiCall(t, handler, token, "GET", "/supporter", nil), http.StatusOK, "Monthly · Level 10", "Yearly · Level 10", "One-time · Level 10", "Show supporter badges around the app")
	for _, edition := range []string{"one-time", "monthly", "yearly"} {
		assertAPIBody(t, apiCall(t, handler, token, "GET", "/api/v1/supporter/certificates/"+edition+".svg", nil), http.StatusOK, "<svg")
	}
	assertAPIBody(t, apiCall(t, handler, token, "GET", "/api/v1/supporter/certificate.svg", nil), http.StatusOK, "Monthly")
}

// Opt-in loopback preview uses synthetic signed certificates and temporary storage.
func TestPreviewThreeEditionSupporter(t *testing.T) {
	if os.Getenv("KINOSAIL_SUPPORTER_PREVIEW") != "1" {
		t.Skip("optional browser preview")
	}
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	activateThreeEditions(t, handler, token, signer)
	preview := &http.Server{Addr: "127.0.0.1:8933", ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
		handler.ServeHTTP(w, r)
	})}
	t.Cleanup(func() { _ = preview.Close() })
	t.Log("preview: http://127.0.0.1:8933/supporter")
	if err := preview.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
