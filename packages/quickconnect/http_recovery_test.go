package quickconnect

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func cancelRequest(t *testing.T, application *Application, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/cancel", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	application.CancelAPI(response, request)
	return response
}

func TestCancelAPIValidatesBeforeRemovingThePendingRequest(t *testing.T) {
	fixture := newApplicationFixture(t)
	secret, _, err := fixture.broker.Create(Request{Numeric: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"", "secret=", "secret=one&secret=two", "secret=%20bad", "secret=bad%0Avalue"} {
		if response := cancelRequest(t, fixture.application, body); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid cancel = %d", response.Code)
		}
		if _, found := fixture.broker.Status(secret); !found {
			t.Fatal("invalid cancel removed unrelated request")
		}
	}
	for range 2 {
		if response := cancelRequest(t, fixture.application, url.Values{"secret": {secret}}.Encode()); response.Code != http.StatusNoContent {
			t.Fatalf("idempotent cancellation = %d", response.Code)
		}
	}
	if _, found := fixture.broker.Status(secret); found {
		t.Fatal("cancel left request pending")
	}
}

func TestCancelAPIRateLimitPreservesPendingState(t *testing.T) {
	fixture := newApplicationFixture(t)
	secret, _, err := fixture.broker.Create(Request{Numeric: true})
	if err != nil {
		t.Fatal(err)
	}
	for range 60 {
		cancelRequest(t, fixture.application, "secret=unrelated")
	}
	response := cancelRequest(t, fixture.application, url.Values{"secret": {secret}}.Encode())
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("unbounded cancellation: %d", response.Code)
	}
	if _, found := fixture.broker.Status(secret); !found {
		t.Fatal("rate-limited cancel changed pending state")
	}
}

func TestApprovalPreviewsRejectUnknownCodesAndInvalidViewers(t *testing.T) {
	fixture := newApplicationFixture(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/quick-connect/123456", nil)
	request.SetPathValue("code", "123456")
	response := httptest.NewRecorder()
	fixture.application.PreviewAPI(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown preview = %d", response.Code)
	}
	response = httptest.NewRecorder()
	serveRemote(fixture.application.PreviewAPI, response, request)
	if response.Code != http.StatusNotFound {
		t.Fatal("public preview exposed approval interface")
	}
	fixture.current = identitycore.Profile{}
	response = httptest.NewRecorder()
	fixture.application.PendingTVsAPI(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatal("anonymous viewer received pending devices")
	}
}

func TestApprovalPageRejectsMalformedAndExpiredLinks(t *testing.T) {
	for _, query := range []string{"?code=bad", "?code=123456"} {
		fixture := newApplicationFixture(t)
		fixture.application.Page(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/quick-connect"+query, nil))
		if fixture.errorStatus != http.StatusBadRequest || fixture.renders != 0 {
			t.Fatal("invalid scanned link reached approval view")
		}
	}
}

func TestApprovalCapacityRejectionDoesNotApproveDevice(t *testing.T) {
	fixture := newApplicationFixture(t)
	secret, pending, err := fixture.broker.Create(Request{Numeric: true})
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
		fixture.limits.Requests.Allow("approve:192.0.2.1", 30)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/approve", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	if err := fixture.application.approveRequest(request, pending.Code); !errors.Is(err, ErrCapacity) || approvalStatus(err) != http.StatusTooManyRequests {
		t.Fatalf("approval capacity = %v", err)
	}
	if _, err := fixture.broker.Consume(secret); !errors.Is(err, ErrPending) {
		t.Fatal("limited approval changed the grant")
	}
}

func TestBrowserStartHandlesBrokerFailureWithoutIssuingCookie(t *testing.T) {
	fixture := newApplicationFixture(t)
	fixture.broker.code = func(bool) (string, error) { return "", errors.New("entropy unavailable") }
	response := browserCall(t, fixture.application.StartBrowser, nil, nil)
	if response.Code != http.StatusBadRequest || len(response.Result().Cookies()) != 0 {
		t.Fatal("failed browser start issued authorization state")
	}
}

func TestPendingTVsUseCodeOrderWhenExpiryMatches(t *testing.T) {
	broker := New(time.Minute)
	now := time.Now()
	broker.now = func() time.Time { return now }
	codes := []string{"654321", "123456"}
	broker.code = func(bool) (string, error) { code := codes[0]; codes = codes[1:]; return code, nil }
	for range 2 {
		if _, _, err := broker.Create(Request{Device: "Kinosail TV", Client: "Kinosail", Numeric: true}); err != nil {
			t.Fatal(err)
		}
	}
	previews, err := broker.PendingTVs(Viewer{ID: "viewer"})
	if err != nil || len(previews) != 2 {
		t.Fatalf("pending = %v %v", previews, err)
	}
	if previews[0].Code != "123456" || previews[1].Code != "654321" {
		t.Fatal("equal expiry yielded unstable device ordering")
	}
}
