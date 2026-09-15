package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// AssertViewerApprovesOneTimeQuickConnectSession runs the original Quick Connect regression against an app.
func AssertViewerApprovesOneTimeQuickConnectSession(t *testing.T, fixture QuickConnectFixture, checkDigits bool) {
	t.Parallel()
	dataDir := t.TempDir()
	handler := fixture.New(dataDir, time.Minute)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	fixture.AddViewer(t, handler, owner)
	viewer := fixture.SignIn(t, handler, "/login", "name=Sam&password=viewer-password")
	start := QuickConnect(t, handler, "/api/v1/quick-connect", "device=Bedroom+TV")
	if checkDigits && (len(start.Code) != 6 || strings.Trim(start.Code, "0123456789") != "") {
		t.Fatalf("Quick Connect code = %q", start.Code)
	}
	pending := QuickConnect(t, handler, "/api/v1/quick-connect/token", "secret="+start.Secret)
	if pending.Token != "" {
		t.Fatalf("pending Quick Connect returned token: %+v", pending)
	}
	approve := fixture.Web(t, handler, http.MethodPost, "/api/v1/quick-connect/"+start.Code, "", viewer)
	if approve.Code != http.StatusNoContent {
		t.Fatalf("approve = %d %q", approve.Code, approve.Body.String())
	}
	connected := QuickConnect(t, handler, "/api/v1/quick-connect/token", "secret="+start.Secret)
	if connected.Token == "" {
		t.Fatalf("approved Quick Connect = %+v", connected)
	}
	api := fixture.APIKey(t, handler, "/api/v1/library", connected.Token)
	if api.Code != http.StatusOK {
		t.Fatalf("Quick Connect session = %d %q", api.Code, api.Body.String())
	}
	reused := QuickConnectRecorder(t, handler, "/api/v1/quick-connect/token", "secret="+start.Secret)
	if reused.Code != http.StatusNotFound {
		t.Fatalf("reused secret = %d %q", reused.Code, reused.Body.String())
	}
}

// AssertApprovedQuickConnectCannotBeReassignedToAnotherViewer runs the original Quick Connect regression against an app.
func AssertApprovedQuickConnectCannotBeReassignedToAnotherViewer(t *testing.T, fixture QuickConnectFixture) {
	t.Parallel()
	handler := fixture.New(t.TempDir(), time.Minute)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	fixture.AddViewer(t, handler, owner)
	viewer := fixture.SignIn(t, handler, "/login", "name=Sam&password=viewer-password")
	started := QuickConnect(t, handler, "/api/v1/quick-connect", "device=Bedroom+TV")

	first := fixture.Web(t, handler, http.MethodPost, "/api/v1/quick-connect/"+started.Code, "", owner)
	retry := fixture.Web(t, handler, http.MethodPost, "/api/v1/quick-connect/"+started.Code, "", owner)
	second := fixture.Web(t, handler, http.MethodPost, "/api/v1/quick-connect/"+started.Code, "", viewer)
	if first.Code != http.StatusNoContent || retry.Code != http.StatusNoContent || second.Code != http.StatusBadRequest {
		t.Fatalf("Quick Connect approvals = first %d %q, retry %d %q, second Viewer %d %q", first.Code, first.Body.String(), retry.Code, retry.Body.String(), second.Code, second.Body.String())
	}

	connected := QuickConnect(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret)
	me := fixture.API(t, handler, connected.Token, http.MethodGet, "/api/v1/me", nil)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"name":"Owner"`) {
		t.Fatalf("approved Viewer was substituted = %d %q", me.Code, me.Body.String())
	}
}

// AssertQuickConnectCodeExpires runs the original Quick Connect regression against an app.
func AssertQuickConnectCodeExpires(t *testing.T, fixture QuickConnectFixture) {
	t.Parallel()
	handler := fixture.New(t.TempDir(), 10*time.Millisecond)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	start := QuickConnect(t, handler, "/api/v1/quick-connect", "device=TV")
	time.Sleep(20 * time.Millisecond)
	response := fixture.Web(t, handler, http.MethodPost, "/quick-connect", "code="+start.Code, owner)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expired code = %d %q", response.Code, response.Body.String())
	}
}

// AssertQuickConnectCreationIsRateLimited runs the original Quick Connect regression against an app.
func AssertQuickConnectCreationIsRateLimited(t *testing.T, fixture QuickConnectFixture) {
	t.Parallel()
	handler := fixture.New(t.TempDir(), 0)
	var response *httptest.ResponseRecorder
	for range 21 {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", strings.NewReader(`{"device":"TV"}`))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "192.0.2.1:1234"
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("Quick Connect limit = %d %v", response.Code, response.Header())
	}
}

// AssertQuickConnectRejectsInvalidDeviceBeforeCreatingRequest runs the original Quick Connect regression against an app.
func AssertQuickConnectRejectsInvalidDeviceBeforeCreatingRequest(t *testing.T, fixture QuickConnectFixture) {
	t.Parallel()
	handler := fixture.New(t.TempDir(), 0)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", strings.NewReader(`{"device":"Living\nRoom"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid device = %d %q", response.Code, response.Body.String())
	}
}
