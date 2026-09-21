package server_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

const supporterActivationID = "00000000-0000-4000-8000-000000000010"

type supporterSigner struct {
	mu              sync.Mutex
	private         ed25519.PrivateKey
	public          string
	now             time.Time
	tier            string
	sustaining      bool
	expiresAt       string
	recognitionName string
	collection      map[string]any
	version         int
	audience        string
	appID           string
	fixtures        map[string]supporterFixture
	activationID    string
	calls           atomic.Int32
	last            map[string]string
}

type supporterFixture struct {
	tier, recognitionName string
	sustaining            bool
	collection            map[string]any
}

func newSupporterSigner(t *testing.T) *supporterSigner {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &supporterSigner{
		private: private, public: base64.RawURLEncoding.EncodeToString(public), now: time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
		tier: "lighthouse", version: 4, audience: "com.kinosail.subtitles", appID: "kino-subtitles",
	}
}

func (signer *supporterSigner) handler(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // The test provider emits one complete certificate fixture.
	signer.mu.Lock()
	defer signer.mu.Unlock()
	signer.calls.Add(1)
	writer.Header().Set("Content-Type", "application/json")
	signer.last = nil
	if err := json.NewDecoder(request.Body).Decode(&signer.last); err != nil {
		http.Error(writer, "invalid", http.StatusBadRequest)
		return
	}
	fixture := supporterFixture{tier: signer.tier, sustaining: signer.sustaining, recognitionName: signer.recognitionName, collection: signer.collection}
	if configured, exists := signer.fixtures[signer.last["key"]]; exists {
		fixture = configured
	}
	expiresAt := any(nil)
	if fixture.sustaining {
		expires := signer.expiresAt
		if expires == "" {
			expires = signer.now.Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
		}
		expiresAt = expires
	}
	certificate := map[string]any{
		"version": signer.version, "audience": signer.audience, "appId": signer.appID, "tier": fixture.tier,
		"supporterId": "A1B2C3D4E5", "supportedSince": signer.now.Add(-400 * 24 * time.Hour).Format(time.RFC3339Nano),
		"issuedAt": signer.now.Format(time.RFC3339Nano), "expiresAt": expiresAt, "sustaining": fixture.sustaining,
		"founding": true, "installationKey": signer.last["installationKey"],
	}
	if signer.version == 4 {
		certificate["family"] = map[bool]string{false: "patron-order", true: "living-standard"}[fixture.sustaining]
		certificate["level"] = supporterTierRank(fixture.tier)
	}
	if signer.version == 2 {
		delete(certificate, "appId")
	}
	if fixture.recognitionName != "" && supporterTierRank(fixture.tier) >= 7 {
		certificate["recognitionName"] = fixture.recognitionName
	}
	if fixture.collection != nil {
		certificate["collection"] = fixture.collection
	}
	record, _ := json.Marshal(certificate)
	activationID := signer.activationID
	if activationID == "" {
		activationID = supporterActivationID
	}
	_ = json.NewEncoder(writer).Encode(map[string]string{
		"certificate": base64.RawURLEncoding.EncodeToString(record), "signature": base64.RawURLEncoding.EncodeToString(ed25519.Sign(signer.private, record)),
		"publicKey": signer.public, "activationId": activationID,
	})
}

func supporterTierRank(tier string) int {
	for index, candidate := range []string{"friend", "crew", "navigator", "patron", "steward", "lighthouse", "commodore", "admiral", "northstar", "legacy"} {
		if tier == candidate {
			return index + 1
		}
	}
	return 0
}

func supporterServer(t *testing.T, dataDir string, signer *supporterSigner, upstream *httptest.Server) (http.Handler, string) {
	factory := func(data, activation, support string, client *http.Client, now func() time.Time) http.Handler {
		return server.New(server.Config{DataDir: data, RequireAuth: true, Supporter: server.SupporterConfig{ActivationURL: activation, SupportURL: support, HTTPClient: client, Now: now}})
	}
	return servertest.SupporterServer(t, dataDir, upstream, func() time.Time { return signer.now }, factory, testTOTP)
}

func TestSupporterBadgeFamiliesUseSpanishAndArabicCatalogs(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	for _, test := range []struct {
		tag  string
		want []string
		dir  string
	}{
		{tag: "es", dir: "ltr", want: []string{"Dos familias de insignias. Cada nivel tiene su propia forma.", "Estandartes Vivientes", "Órdenes del Mecenas", "Corona Rosetta"}},
		{tag: "ar", dir: "rtl", want: []string{"عائلتان من الشارات. لكل مستوى شكل مستقل.", "الرايات الحية", "أوسمة الرعاة", "تاج رشيد"}},
	} {
		t.Run(test.tag, func(t *testing.T) {
			page := apiCall(t, handler, token, http.MethodGet, "/supporter?lang="+test.tag, nil)
			if page.Code != http.StatusOK || page.Header().Get("Content-Language") != test.tag || !strings.Contains(page.Body.String(), `<html lang="`+test.tag+`" dir="`+test.dir+`">`) {
				t.Fatalf("localized supporter shell = %d, language %q, body %q", page.Code, page.Header().Get("Content-Language"), page.Body.String())
			}
			for _, want := range test.want {
				if !strings.Contains(page.Body.String(), want) {
					t.Fatalf("localized supporter page missing %q: %q", want, page.Body.String())
				}
			}
		})
	}
}

func TestSupporterStoresPatronOrderAndLivingStandardIndependently(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	data := t.TempDir()
	handler, token := supporterServer(t, data, signer, upstream)

	patron := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "PATRON_SUPPORTER_KEY"})
	assertAPIBody(t, patron, http.StatusOK, `"tier":"lighthouse"`, `"sustaining":false`, `"app":{"id":"kino-subtitles","name":"Kinosail Subtitles"}`, `"patronOrder":{"family":"patron-order"`, `"serviceMonths":0`, `"livingStandard":{"family":"living-standard"`, `"tier":"free"`)
	if signer.last["appId"] != "kino-subtitles" || len(signer.last["installationKey"]) != 43 || len(signer.last) != 3 {
		t.Fatalf("activation request = %#v", signer.last)
	}
	patronCertificate := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter/certificates/patron-order", nil)
	if patronCertificate.Code != http.StatusOK || strings.Contains(patronCertificate.Body.String(), "LIVING SERVICE") {
		t.Fatalf("Patron certificate included Living tenure: %d %q", patronCertificate.Code, patronCertificate.Body.String())
	}

	signer.tier, signer.sustaining = "admiral", true
	living := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "MONTHLY_SUPPORTER_KEY"})
	assertAPIBody(t, living, http.StatusOK, `"tier":"admiral"`, `"sustaining":true`, `"livingStandard":{"family":"living-standard"`, `"rank":8`, `"active":true`, `"serviceMarks":[3,6,12]`, `"patronOrder":{"family":"patron-order"`, `"tier":"lighthouse"`, `"livingLevel":8`, `"patronLevel":6`, `"masterworkLevel":6`, `"unlocked":14`, `"masterworkName":"Perfect Sync"`, `"masterworkEarned":true`, `"masterworkActive":true`)
	page := apiCall(t, handler, token, http.MethodGet, "/supporter", nil)
	assertAPIBody(t, page, http.StatusOK, "Monthly support · Recommended", "Living Standards", "Patron Orders", "Polyglot Array", "Linguist Crest", "Kinosail Subtitles", "caption emblem", "Download SVG certificate", "Share PNG certificate", "Perfect Sync · Level 6", "Caption Concord", "Living cadence active", "Collected", "/static/app.css?v=impeccable-1")
	if strings.Contains(page.Body.String(), "/static/app.css?v=electric-1") {
		t.Fatal("supporter page retained the previous combined stylesheet key")
	}

	restarted := server.New(server.Config{DataDir: data, RequireAuth: true, Supporter: server.SupporterConfig{ActivationURL: upstream.URL, HTTPClient: upstream.Client(), Now: func() time.Time { return signer.now }}})
	assertAPIBody(t, apiCall(t, restarted, token, http.MethodGet, "/api/v1/supporter", nil), http.StatusOK, `"livingStandard":{"family":"living-standard"`, `"tier":"admiral"`, `"patronOrder":{"family":"patron-order"`, `"tier":"lighthouse"`)
}

func TestSupporterKeepsHighestPatronAndReplacesEqualRank(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.tier = "legacy"
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LEGACY_PATRON_KEY"}), http.StatusOK, `"tier":"legacy"`)

	signer.tier = "crew"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "LOWER_PATRON_KEY"}), http.StatusOK, `"tier":"legacy"`, `"patronOrder":{"family":"patron-order"`)

	signer.tier = "legacy"
	signer.collection = map[string]any{"id": "complete-fleet", "name": "Complete Fleet", "edition": "2026 Founders", "appIds": []string{"kino-player", "kino-subtitles"}}
	status := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "FLEET_PATRON_KEY"})
	assertAPIBody(t, status, http.StatusOK, `"tier":"legacy"`, `"sustaining":false`, `"collection":{"id":"complete-fleet","name":"Complete Fleet","edition":"2026 Founders"`)

	signer.collection = nil
	status = apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "APP_ONLY_PATRON_KEY"})
	assertAPIBody(t, status, http.StatusOK, `"tier":"legacy"`, `"collection":{"id":"complete-fleet","name":"Complete Fleet","edition":"2026 Founders"`)
}

func TestSupporterAcceptsLowerLevelExplicitNameWhenServiceOmitsIt(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.tier, signer.recognitionName = "crew", "Public Name"
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	status := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": "CREW_SUPPORT_KEY", "recognitionName": "Public Name"})
	assertAPIBody(t, status, http.StatusOK, `"tier":"crew"`, `"patronOrder":{"family":"patron-order"`)
	if signer.last["recognitionName"] != "Public Name" || strings.Contains(status.Body.String(), "recognitionName") {
		t.Fatalf("lower-level recognition = request %#v, response %q", signer.last, status.Body.String())
	}
}

func TestSupporterActivationRejectsSignedV2WithoutStateChange(t *testing.T) {
	for _, refresh := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "refresh"}[refresh], func(t *testing.T) {
			signer := newSupporterSigner(t)
			upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
			defer upstream.Close()
			handler, token := supporterServer(t, t.TempDir(), signer, upstream)
			key := "VERSIONED_SUPPORTER_KEY"
			if refresh {
				assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": key}), http.StatusOK, `"tier":"lighthouse"`)
			}
			before := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String()
			signer.version = 2
			failed := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": key})
			assertAPIBody(t, failed, http.StatusBadRequest, `"error":"supporter activation returned an invalid certificate"`)
			if after := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String(); after != before {
				t.Fatalf("v2 activation changed state: before %s, after %s", before, after)
			}
			if (signer.last["activationId"] != "") != refresh {
				t.Fatalf("v2 activation request = %#v", signer.last)
			}
		})
	}
}

func TestSupporterRefreshRejectsChangedActivationIDWithoutStateChange(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	key := "REFRESH_SUPPORTER_KEY"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": key}), http.StatusOK, `"tier":"lighthouse"`)
	before := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String()

	signer.activationID = "00000000-0000-4000-8000-000000000011"
	failed := apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": key})
	assertAPIBody(t, failed, http.StatusBadRequest, `"error":"supporter activation returned an invalid response"`)
	if signer.last["activationId"] != supporterActivationID || apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil).Body.String() != before {
		t.Fatalf("changed refresh ID altered state: request %#v", signer.last)
	}

	signer.activationID = supporterActivationID
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": key}), http.StatusOK, `"tier":"lighthouse"`)
	if signer.last["activationId"] != supporterActivationID {
		t.Fatalf("saved activation ID changed after rejection: %#v", signer.last)
	}
}
