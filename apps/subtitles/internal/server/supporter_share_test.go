package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSupporterCompleteFleetShareCertificateIsPublicSafe(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.tier, signer.sustaining, signer.recognitionName = "legacy", true, "Quiet Benefactor"
	signer.collection = map[string]any{"id": "complete-fleet", "name": "Complete Fleet", "edition": "Living", "appIds": []string{"kino-player", "kino-subtitles"}}
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	activated := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "FLEET_SUPPORTER_KEY", "recognitionName": "Quiet Benefactor"})
	assertAPIBody(t, activated, http.StatusOK, `"recognitionName":"Quiet Benefactor"`, `"collection":{"id":"complete-fleet","name":"Complete Fleet","edition":"Living","appIds":["kino-player","kino-subtitles"]}`)
	if signer.last["recognitionName"] != "Quiet Benefactor" {
		t.Fatalf("recognition request = %#v", signer.last)
	}
	page := apiCall(t, handler, token, http.MethodGet, "/supporter", nil)
	assertAPIBody(t, page, http.StatusOK, "Complete Fleet · Living", "2 apps in this signed edition", "follows future configured apps while active")
	certificate := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/living-standard", nil)
	assertAPIBody(t, certificate, http.StatusOK, "Rosetta Crown", "Quiet Benefactor", "LIVING SERVICE / 3M · 6M · 12M", "COMPLETE FLEET / Living", "KINOSAIL SUBTITLES / ACTIVE")
	if got := certificate.Header().Get("Content-Type"); !strings.HasPrefix(got, "image/svg+xml") {
		t.Fatalf("certificate content type = %q", got)
	}
	for _, private := range []string{"A1B2C3D4E5", signer.last["installationKey"], "FLEET_SUPPORTER_KEY", supporterActivationID, "signature", "publicKey"} {
		if strings.Contains(certificate.Body.String(), private) {
			t.Fatalf("share certificate exposed %q", private)
		}
	}
	if evidenceDir := os.Getenv("KINOSAIL_SUPPORTER_EVIDENCE_DIR"); evidenceDir != "" {
		if err := os.MkdirAll(evidenceDir, 0o750); err != nil { //nolint:gosec // Evidence output is explicitly configured by the test environment.
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, "living-standard-level-10.svg"), certificate.Body.Bytes(), 0o600); err != nil { //nolint:gosec // Evidence output is explicitly configured by the test environment.
			t.Fatal(err)
		}
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/unknown", nil), http.StatusNotFound)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/unknown?extra=true", nil), http.StatusBadRequest, "invalid supporter request")
}

func TestSupporterShareCertificateFitsMaximumRecognitionName(t *testing.T) {
	name := strings.Repeat("W", 80)
	signer := newSupporterSigner(t)
	signer.tier, signer.recognitionName = "legacy", name
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "MAX_NAME_SUPPORTER_KEY", "recognitionName": name}), http.StatusOK, `"recognitionName":"`+name+`"`)
	certificate := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/patron-order", nil)
	assertAPIBody(t, certificate, http.StatusOK, `textLength="570" lengthAdjust="spacingAndGlyphs">`+name+`</text>`)
}

func TestSupporterRecognitionNameEscapesHTMLAndSVG(t *testing.T) {
	name := `A <B> & "C"`
	signer := newSupporterSigner(t)
	signer.tier, signer.recognitionName = "legacy", name
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "ESCAPED_NAME_SUPPORTER_KEY", "recognitionName": name}), http.StatusOK)
	for path, label := range map[string]string{"/supporter": "HTML", "/api/v1/supporter/certificates/patron-order": "SVG"} {
		response := apiCall(t, handler, token, http.MethodGet, path, nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `A &lt;B&gt; &amp; &#34;C&#34;`) || strings.Contains(response.Body.String(), name) {
			t.Fatalf("%s recognition escaping = %d %q", label, response.Code, response.Body.String())
		}
	}
}

func TestExpiredLivingStandardIsArchivedNotActive(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.tier, signer.sustaining = "commodore", true
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "MONTHLY_SUPPORTER_KEY"}), http.StatusOK, `"active":true`)
	signer.now = signer.now.Add(31 * 24 * time.Hour)
	status := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil)
	assertAPIBody(t, status, http.StatusOK, `"tier":"commodore"`, `"active":false`, `"expired":true`, `"sustaining":true`, `"livingStandard":{"family":"living-standard"`, `"archived":true`)
	page := apiCall(t, handler, token, http.MethodGet, "/supporter", nil)
	assertAPIBody(t, page, http.StatusOK, "Elite level · Archived", "Active through")
	certificate := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/living-standard", nil)
	assertAPIBody(t, certificate, http.StatusOK, "KINOSAIL SUBTITLES / ARCHIVED", "ACTIVE THROUGH")
}
