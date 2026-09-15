package jellyfincompat

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/quickconnect"
)

func TestQuickConnectRequestRequiresAndNormalizesIdentity(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Initiate", nil)
	request.Header.Set("Authorization", `MediaBrowser Client="Swiftfin", Device="  Living   Room  ", DeviceId="tv-1", Version="1.2"`)
	identity, valid := QuickConnectRequest(request, true)
	want := quickconnect.Request{Device: "Living   Room", DeviceID: "tv-1", Client: "Swiftfin", Version: "1.2", Numeric: true, Remote: true}
	if !valid || !reflect.DeepEqual(identity, want) {
		t.Fatalf("identity = %#v, valid %t", identity, valid)
	}
	missing, valid := QuickConnectRequest(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), false)
	if valid || missing.Numeric != true || missing.Remote {
		t.Fatalf("missing identity = %#v, valid %t", missing, valid)
	}
	conflicting := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	conflicting.Header.Set("Authorization", `MediaBrowser Client="Swiftfin", Client="Other", Device="TV", DeviceId="tv-1", Version="1"`)
	if _, valid := QuickConnectRequest(conflicting, false); valid {
		t.Fatal("conflicting device identity was accepted")
	}
}

func TestQuickConnectDTOProjectsEveryPlayerField(t *testing.T) {
	created := time.Unix(2_000, 0).UTC()
	dto := QuickConnectDTO("secret", quickconnect.Connection{Code: "123456", Device: "TV", DeviceID: "tv-1", Client: "Swiftfin", Version: "1", Created: created, Approved: true})
	want := map[string]any{"Authenticated": true, "Secret": "secret", "Code": "123456", "DateAdded": created, "DeviceId": "tv-1", "DeviceName": "TV", "AppName": "Swiftfin", "AppVersion": "1"}
	if !reflect.DeepEqual(dto, want) {
		t.Fatalf("Quick Connect DTO = %#v", dto)
	}
}

func TestStartQuickConnectPreservesValidationRateAndCreationErrors(t *testing.T) { //nolint:cyclop,funlen,gocognit // One lifecycle verifies ordering and exact HTTP translations.
	connections, limiter := quickconnect.New(time.Minute), &httpguard.Limiter{}
	missing := httptest.NewRecorder()
	StartQuickConnect(missing, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), connections, limiter, false)
	if missing.Code != http.StatusBadRequest || missing.Body.String() != "device identity is required\n" || limiter.TrackedClients() != 0 {
		t.Fatalf("missing identity = %d %q, tracked %d", missing.Code, missing.Body.String(), limiter.TrackedClients())
	}
	conflictingRequest := jellyfinIdentityRequest(t, "Client", "device")
	conflictingRequest.Header.Set("X-Emby-Authorization", `MediaBrowser Client="Other"`)
	conflicting := httptest.NewRecorder()
	StartQuickConnect(conflicting, conflictingRequest, connections, limiter, false)
	if conflicting.Code != http.StatusBadRequest || limiter.TrackedClients() != 0 {
		t.Fatalf("conflicting identity = %d, tracked %d", conflicting.Code, limiter.TrackedClients())
	}
	request := jellyfinIdentityRequest(t, "Swiftfin", "tv-1")
	success := httptest.NewRecorder()
	StartQuickConnect(success, request, connections, limiter, true)
	if success.Code != http.StatusOK || !strings.Contains(success.Body.String(), `"Authenticated":false`) || !strings.Contains(success.Body.String(), `"DeviceName":"TV"`) {
		t.Fatalf("start = %d %q", success.Code, success.Body.String())
	}
	invalid := httptest.NewRecorder()
	StartQuickConnect(invalid, jellyfinIdentityRequest(t, strings.Repeat("c", 81), "tv-2"), connections, &httpguard.Limiter{}, false)
	if invalid.Code != http.StatusBadRequest || invalid.Body.String() != "device identity is invalid\n" {
		t.Fatalf("invalid identity = %d %q", invalid.Code, invalid.Body.String())
	}
	invalidDeviceRequest := jellyfinIdentityRequest(t, "Client", "tv-3")
	invalidDeviceRequest.Header.Set("Authorization", `MediaBrowser Client="Client", Device="`+strings.Repeat("d", 81)+`", DeviceId="tv-3", Version="1"`)
	invalidDeviceLimiter := &httpguard.Limiter{}
	invalidDevice := httptest.NewRecorder()
	StartQuickConnect(invalidDevice, invalidDeviceRequest, connections, invalidDeviceLimiter, false)
	if invalidDevice.Code != http.StatusBadRequest || invalidDeviceLimiter.TrackedClients() != 0 {
		t.Fatalf("invalid device = %d, tracked %d", invalidDevice.Code, invalidDeviceLimiter.TrackedClients())
	}
	rateConnections, rateLimiter := quickconnect.New(time.Minute), &httpguard.Limiter{}
	for index := 0; index < 21; index++ {
		response := httptest.NewRecorder()
		StartQuickConnect(response, jellyfinIdentityRequest(t, "Client", "device"), rateConnections, rateLimiter, false)
		if index == 20 && (response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" || response.Body.String() != "too many Quick Connect requests\n") {
			t.Fatalf("rate limit = %d %q %v", response.Code, response.Body.String(), response.Header())
		}
	}
	full := quickconnect.New(time.Minute)
	for index := 0; index < 1024; index++ {
		if _, _, err := full.Create(quickconnect.Request{Device: "TV", DeviceID: "id", Client: "client", Version: "1"}); err != nil {
			t.Fatalf("fill request %d: %v", index, err)
		}
	}
	capacity := httptest.NewRecorder()
	StartQuickConnect(capacity, jellyfinIdentityRequest(t, "Client", "device"), full, &httpguard.Limiter{}, false)
	if capacity.Code != http.StatusInternalServerError || capacity.Body.String() != "could not create Quick Connect request\n" {
		t.Fatalf("capacity = %d %q", capacity.Code, capacity.Body.String())
	}
}

func TestQuickConnectStatusPreservesPollingContract(t *testing.T) { //nolint:cyclop // Exact status fields protect the complete polling wire contract.
	connections := quickconnect.New(time.Minute)
	secret, _, err := connections.Create(quickconnect.Request{Device: "TV", DeviceID: "id", Client: "client", Version: "1"})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Connect?Secret="+secret, nil)
	QuickConnectStatus(response, request, connections, &httpguard.Limiter{})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"Secret":"`+secret+`"`) {
		t.Fatalf("status = %d %q", response.Code, response.Body.String())
	}
	missing := httptest.NewRecorder()
	QuickConnectStatus(missing, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Connect", nil), connections, &httpguard.Limiter{})
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d %q", missing.Code, missing.Body.String())
	}
	unknown := httptest.NewRecorder()
	QuickConnectStatus(unknown, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Connect?secret=qc_BBBBBBBBBBBBBBBBBBBBBBBBBB", nil), connections, &httpguard.Limiter{})
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown status = %d %q", unknown.Code, unknown.Body.String())
	}
	conflicting := httptest.NewRecorder()
	QuickConnectStatus(conflicting, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Connect?secret=one&Secret=two", nil), connections, &httpguard.Limiter{})
	if conflicting.Code != http.StatusNotFound {
		t.Fatalf("conflicting status = %d %q", conflicting.Code, conflicting.Body.String())
	}
	limiter := &httpguard.Limiter{}
	for index := 0; index < 120; index++ {
		limiter.Allow("192.0.2.1", 120)
	}
	limited := httptest.NewRecorder()
	limitedRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/QuickConnect/Connect?secret="+secret, nil)
	limitedRequest.RemoteAddr = "192.0.2.1:1234"
	QuickConnectStatus(limited, limitedRequest, connections, limiter)
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") != "60" || limited.Body.String() != "too many Quick Connect polls\n" {
		t.Fatalf("limited status = %d %q %v", limited.Code, limited.Body.String(), limited.Header())
	}
}

func TestAuthenticateQuickConnectPreservesJSONAndSessionContract(t *testing.T) { //nolint:cyclop // One table covers parser, failure, success, and rate ordering.
	approvedSecret := "qc_AAAAAAAAAAAAAAAAAAAAAAAAAA"
	missingSecret := "qc_BBBBBBBBBBBBBBBBBBBBBBBBBB"
	callback := func(secret string) (Authentication, error) {
		if secret == missingSecret {
			return Authentication{}, errors.New("not found")
		}
		return Authentication{Token: "token", User: User{ID: "viewer", Name: "Sam", ServerID: "server"}}, nil
	}
	for _, test := range []struct {
		body      string
		status    int
		bodyPart  string
		bodyExact string
	}{
		{`{"Secret":"` + approvedSecret + `"}`, http.StatusOK, `"AccessToken":"token"`, ""},
		{`{"Secret":"` + missingSecret + `"}`, http.StatusNotFound, "", "404 page not found\n"},
		{`{"Secret":"wrong"}`, http.StatusNotFound, "", "404 page not found\n"},
		{`{`, http.StatusBadRequest, "", "{\"error\":\"invalid JSON request\"}\n"},
		{`{"Secret":"` + approvedSecret + `","Unknown":true}`, http.StatusBadRequest, "", "{\"error\":\"invalid JSON request\"}\n"},
		{`{"Secret":"` + approvedSecret + `"}{}`, http.StatusBadRequest, "", "{\"error\":\"request must contain one JSON object\"}\n"},
	} {
		response := httptest.NewRecorder()
		AuthenticateQuickConnect(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(test.body)), &httpguard.Limiter{}, callback)
		if response.Code != test.status || test.bodyExact != "" && response.Body.String() != test.bodyExact || test.bodyPart != "" && !strings.Contains(response.Body.String(), test.bodyPart) {
			t.Errorf("body %q = %d %q", test.body, response.Code, response.Body.String())
		}
	}
	limiter := &httpguard.Limiter{}
	for index := 0; index < 120; index++ {
		limiter.Allow("192.0.2.1", 120)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{`))
	request.RemoteAddr = "192.0.2.1:1234"
	AuthenticateQuickConnect(response, request, limiter, callback)
	if response.Code != http.StatusTooManyRequests || response.Body.String() != "too many Quick Connect polls\n" || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("rate-limited authentication = %d %q %v", response.Code, response.Body.String(), response.Header())
	}
}

func TestWriteQuickConnectApprovalPreservesCanonicalResponses(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		body   string
	}{
		{nil, http.StatusOK, "true\n"},
		{ErrViewerUnavailable, http.StatusForbidden, ErrViewerUnavailable.Error() + "\n"},
		{quickconnect.ErrRemoteViewer, http.StatusForbidden, quickconnect.ErrRemoteViewer.Error() + "\n"},
		{ErrPublicApproval, http.StatusNotFound, "404 page not found\n"},
		{quickconnect.ErrInvalidCode, http.StatusNotFound, "404 page not found\n"},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Authorize", nil)
		WriteQuickConnectApproval(response, request, test.err)
		if response.Code != test.status || response.Body.String() != test.body {
			t.Errorf("approval %v = %d %q", test.err, response.Code, response.Body.String())
		}
	}
}

func jellyfinIdentityRequest(t *testing.T, client, deviceID string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Initiate", nil)
	request.Header.Set("Authorization", `MediaBrowser Client="`+client+`", Device="TV", DeviceId="`+deviceID+`", Version="1"`)
	return request
}
