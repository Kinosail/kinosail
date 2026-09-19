package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSupporterStoresIndependentFirstClassBadges(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.collection = map[string]any{"id": "complete-fleet", "name": "Complete Fleet", "edition": "Living", "appIds": []string{"kino-player", "kino-subtitles"}}
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)

	free := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil)
	assertAPIBody(t, free, http.StatusOK, `"appId":"kino-player"`, `"tier":"free"`, `"active":false`, `"supportUrl":"https://support.example"`)
	freePage := apiCall(t, handler, token, http.MethodGet, "/supporter", nil)
	if !strings.HasPrefix(freePage.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("supporter page media type = %q", freePage.Header().Get("Content-Type"))
	}
	assertAPIBody(t, freePage, http.StatusOK, "One Complete Fleet key covers every included app", "Each app keeps its own emblem and certificate", "Living editions include future configured apps while active", "Dated Patron editions stay fixed")
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/static/supporter.js", nil), http.StatusOK, "FileReader", "image/svg+xml", "262144", "image/png", "navigator.share")
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/static/supporter/badges/supporter-rank-10.svg", nil), http.StatusOK, "<svg", "<path", "Sovereign Full Sail")
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/static/supporter/badges/supporter-rank-10.png", nil), http.StatusNotFound, "404 page not found")
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/living-standard.svg", nil), http.StatusNotFound, "supporter badge is not available")

	living := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LIVING_STANDARD_KEY", "recognitionName": "Private Patron"})
	assertAPIBody(t, living, http.StatusOK, `"family":"living-standard"`, `"tier":"legacy"`, `"rank":10`, `"active":true`, `"serviceMarks":[3,6,12]`, `"completeFleetActive":true`, `"livingLevel":10`, `"patronLevel":0`, `"unlocked":10`)
	assertLivingActivationRequest(t, signer.last)

	signer.sustaining, signer.expiresAt, signer.tier, signer.collection = false, nil, "admiral", nil
	patron := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "PATRON_ORDER_KEY"})
	assertAPIBody(t, patron, http.StatusOK, `"family":"patron-order"`, `"tier":"admiral"`, `"rank":8`, `"livingStandard"`, `"patronOrder"`, `"subscriptionActive":true`, `"masterworkName":"Full Sail"`, `"masterworkLevel":8`, `"masterworkEarned":true`, `"masterworkActive":true`)

	page := apiCall(t, handler, token, http.MethodGet, "/supporter", nil)
	assertAPIBody(t, page, http.StatusOK, "/static/app.css?v=electric-14", "Monthly support · Living Standard", "Living Standards", "Patron Orders", "Friend", "Legacy", "Navigator", "North Star", "Kinosail Player Living Standard, level 10", "Kinosail Player Patron Order, level 8", "Admiral Full Sail", "Living aura active", "Collected", "Complete Fleet · Living · 2 apps", "Living editions include future configured apps", "Dated Patron editions stay fixed", "Public certificate name", "levels 7–10", "Your one-time badge is permanent.")
	if strings.Index(page.Body.String(), "Living Standards") > strings.Index(page.Body.String(), "Patron Orders") {
		t.Fatal("monthly Living Standards did not appear first")
	}

	livingCertificate := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/living-standard.svg", nil)
	assertAPIBody(t, livingCertificate, http.StatusOK, "Private Patron", "Living Standard · LEVEL 10", "Legacy Constellation", "Complete Fleet · Living", "Tenure marks · 3 / 6 / 12 months", "Kinosail Player", "Sailcraft — Legacy — Living Standard")
	patronCertificate := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/patron-order.svg", nil)
	assertAPIBody(t, patronCertificate, http.StatusOK, "Kinosail Supporter", "Patron Order · LEVEL 8", "Admiral Star", "Permanent", "Sailcraft — Admiral — Patron Order")
	if strings.Contains(patronCertificate.Body.String(), "Tenure marks") {
		t.Fatal("Patron certificate showed Living tenure marks")
	}
	if strings.Contains(livingCertificate.Body.String(), "Sailcraft — Admiral — Patron Order") || strings.Contains(patronCertificate.Body.String(), "Sailcraft — Legacy — Living Standard") {
		t.Fatal("certificate family and level reused another badge outline")
	}
	legacyCertificate := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificate.svg", nil)
	assertAPIBody(t, legacyCertificate, http.StatusOK, "Legacy Constellation")
	assertSupporterCertificatePrivacy(t, signer.last["installationKey"], livingCertificate, patronCertificate, legacyCertificate)
	assertSupporterScriptRecognition(t, handler)
}

func TestSupporterScriptUsesOneImmutableVersionAcrossPages(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	for _, path := range []string{"/", "/settings", "/supporter"} {
		page := apiCall(t, handler, token, http.MethodGet, path, nil)
		assertAPIBody(t, page, http.StatusOK, `/static/supporter.js?v=11`)
		if strings.Count(page.Body.String(), `/static/supporter.js?v=`) != 1 {
			t.Errorf("page %q supporter script count = %d", path, strings.Count(page.Body.String(), `/static/supporter.js?v=`))
		}
	}
}

func TestSupporterArchivesExpiredLivingStandardWithoutReplacingPatronOrder(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.expiresAt = signer.now.Add(24 * time.Hour).Format(time.RFC3339Nano)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LIVING_STANDARD_KEY"}), http.StatusOK, `"subscriptionActive":true`)
	signer.sustaining, signer.expiresAt, signer.tier = false, nil, "commodore"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "PATRON_ORDER_KEY"}), http.StatusOK, `"patronOrder"`)
	signer.now = signer.now.Add(48 * time.Hour)
	status := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil)
	assertAPIBody(t, status, http.StatusOK, `"family":"living-standard"`, `"expired":true`, `"family":"patron-order"`, `"active":true`, `"subscriptionActive":false`)
	var projection struct {
		Tier       string `json:"tier"`
		ExpiresAt  string `json:"expiresAt"`
		Expired    bool   `json:"expired"`
		Sustaining bool   `json:"sustaining"`
	}
	mustJSON(t, status, &projection)
	if projection.Tier != "commodore" || projection.ExpiresAt != "" || projection.Expired || projection.Sustaining {
		t.Fatalf("flat supporter projection combined badge families: %#v", projection)
	}
	page := apiCall(t, handler, token, http.MethodGet, "/supporter", nil)
	assertAPIBody(t, page, http.StatusOK, "Archived after", "is-archived", "Patron Order · Level 7")
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/living-standard.svg", nil), http.StatusOK, "Archived · Supported through")
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificate.svg", nil), http.StatusOK, "Commodore Order")
}

func TestSupporterEqualRankPatronOrderKeepsCompleteFleet(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.sustaining, signer.expiresAt, signer.tier = false, nil, "admiral"
	signer.collection = map[string]any{"id": "complete-fleet", "name": "Complete Fleet", "edition": "2026", "appIds": []string{"kino-player", "kino-subtitles"}}
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "FLEET_PATRON_KEY"}), http.StatusOK, `"edition":"2026"`)
	signer.collection = nil
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "APP_ONLY_PATRON_KEY"}), http.StatusOK, `"edition":"2026"`, `"appIds":["kino-player","kino-subtitles"]`)
}

func TestSupporterRefreshRejectsChangedUnsignedActivationID(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "REFRESH_SUPPORTER_KEY"}), http.StatusOK, `"tier":"legacy"`)
	before := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String()
	signer.activationID = "00000000-0000-4000-8000-000000000011"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "REFRESH_SUPPORTER_KEY"}), http.StatusBadRequest, "invalid response")
	if signer.last["activationId"] != supporterActivationID {
		t.Fatalf("refresh activation id = %#v", signer.last)
	}
	after := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String()
	if after != before {
		t.Fatalf("changed refresh activation id replaced state: before %q, after %q", before, after)
	}
}

func TestSupporterRecognitionIsExplicitAndLowerLevelsIgnoreIt(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.tier = "crew"
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	response := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LOW_LEVEL_KEY", "recognitionName": "Explicit Name"})
	assertAPIBody(t, response, http.StatusOK, `"tier":"crew"`)
	if strings.Contains(response.Body.String(), "Explicit Name") || signer.last["recognitionName"] != "Explicit Name" {
		t.Fatalf("lower-level recognition handling = %q, request %#v", response.Body.String(), signer.last)
	}
	before := signer.calls.Load()
	invalid := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LOW_LEVEL_KEY", "recognitionName": "Hidden\u200bName"})
	assertAPIBody(t, invalid, http.StatusBadRequest, "invalid supporter activation request")
	if signer.calls.Load() != before {
		t.Fatal("invalid recognition name reached provider")
	}
}

func TestSupporterRejectsUnrequestedPublicRecognitionName(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.recognitionName = "Injected Alias"
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "PRIVATE_SUPPORTER_KEY"}), http.StatusBadRequest, "invalid certificate")
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil), http.StatusOK, `"tier":"free"`, `"active":false`)
}

func TestSupporterRejectsInvalidV3CertificatesWithoutChangingState(t *testing.T) {
	tests := map[string]func(*supporterSigner){
		"wrong audience":       func(s *supporterSigner) { s.audience = "com.kinosail.subtitles" },
		"wrong app":            func(s *supporterSigner) { s.appID = "kino-subtitles" },
		"unknown field":        func(s *supporterSigner) { s.extraField = true },
		"missing required":     func(s *supporterSigner) { s.omitField = "appId" },
		"one-time expiry":      func(s *supporterSigner) { s.sustaining = false },
		"subscription no date": func(s *supporterSigner) { s.expiresAt = nil },
		"collection null":      func(s *supporterSigner) { s.collection = json.RawMessage("null") },
		"collection unsorted": func(s *supporterSigner) {
			s.collection = map[string]any{"id": "complete-fleet", "name": "Complete Fleet", "edition": "2026", "appIds": []string{"kino-subtitles", "kino-player"}}
		},
		"collection no player": func(s *supporterSigner) {
			s.collection = map[string]any{"id": "complete-fleet", "name": "Complete Fleet", "edition": "2026", "appIds": []string{"kino-books", "kino-subtitles"}}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			signer := newSupporterSigner(t)
			mutate(signer)
			upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
			defer upstream.Close()
			handler, token := supporterServer(t, t.TempDir(), signer, upstream)
			assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "INVALID_CERTIFICATE_KEY"}), http.StatusBadRequest, "invalid certificate")
			status := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil)
			assertAPIBody(t, status, http.StatusOK, `"tier":"free"`, `"active":false`)
			if strings.Contains(status.Body.String(), "livingStandard") || strings.Contains(status.Body.String(), "patronOrder") {
				t.Fatalf("invalid certificate changed local state: %q", status.Body.String())
			}
		})
	}
}

func TestSupporterRejectsNonCanonicalActivationIdentifiers(t *testing.T) {
	for name, activationID := range map[string]string{
		"uppercase": "00000000-0000-4000-8000-00000000001A",
		"non-v4":    "00000000-0000-1000-8000-000000000010",
	} {
		t.Run(name, func(t *testing.T) {
			signer := newSupporterSigner(t)
			signer.activationID = activationID
			upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
			defer upstream.Close()
			handler, token := supporterServer(t, t.TempDir(), signer, upstream)
			assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "INVALID_ACTIVATION_KEY"}), http.StatusBadRequest, "invalid response")
			assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil), http.StatusOK, `"tier":"free"`, `"active":false`)
		})
	}
}

func TestSupporterInputValidationAndStatusPrivacy(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	for _, body := range []map[string]any{{}, {"key": "short"}, {"key": strings.Repeat("A", 129)}, {"key": "VALID_SUPPORTER_KEY", "unexpected": true}, {"key": "VALID_SUPPORTER_KEY", "recognitionName": strings.Repeat("A", 81)}} {
		if response := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", body); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid activation %#v = %d %q", body, response.Code, response.Body.String())
		}
	}
	if signer.calls.Load() != 0 {
		t.Fatalf("invalid input reached provider %d times", signer.calls.Load())
	}
	assertAmbiguousSupporterJSON(t, handler, token)
	assertInvalidSupporterAPIBoundaries(t, handler, token)
	for _, path := range []string{"/api/v1/supporter/certificate.svg?family=patron", "/api/v1/supporter/certificates/patron-order.svg?download=true", "/api/v1/supporter/certificates/living-standard.svg?download=true"} {
		assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, path, nil), http.StatusBadRequest, "must not contain parameters")
	}
	assertInvalidSupporterForms(t, handler, token)
	if signer.calls.Load() != 0 {
		t.Fatalf("ambiguous input reached provider %d times", signer.calls.Load())
	}
	if viewer := apiCall(t, handler, "", http.MethodGet, "/api/v1/supporter", nil); viewer.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous supporter status = %d %q", viewer.Code, viewer.Body.String())
	}
}

func assertSupporterScriptRecognition(t *testing.T, handler http.Handler) {
	t.Helper()
	script := apiCall(t, handler, "", http.MethodGet, "/static/supporter.js", nil)
	assertAPIBody(t, script, http.StatusOK, "Community edition · Free and supporter-funded.", "supporter-signature", "supporter-level-logo", "/static/supporter/badges/", "htmx:afterSwap", "supporter-edition-mark", "supporterShareFile", "navigator.share", "image/png")
	for _, dismissal := range []string{"localStorage", "Dismiss", "/supporter/reminder"} {
		if strings.Contains(script.Body.String(), dismissal) {
			t.Fatalf("supporter script contains dismissal path %q", dismissal)
		}
	}
}
