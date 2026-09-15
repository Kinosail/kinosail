package quickconnect

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTVPreviewsAreLocalBoundedAndContainNoSecrets(t *testing.T) { //nolint:cyclop // One preview response is checked for locality, bounds, and secret redaction.
	t.Parallel()
	broker := New(time.Minute)
	viewer := Viewer{ID: "viewer"}
	secrets := make([]string, 0)
	for range 10 {
		secret, _, err := broker.Create(Request{Device: "Kinosail TV", Client: "Kinosail", Numeric: true})
		if err != nil {
			t.Fatal(err)
		}
		secrets = append(secrets, secret)
	}
	for _, request := range []Request{
		{Device: "Kinosail TV", Client: "Kinosail", Numeric: true, Remote: true},
		{Device: "Kinosail Player", Client: "Kinosail", Numeric: true},
		{Device: "Kinosail TV", Client: "Other client", Numeric: true},
	} {
		_, _, _ = broker.Create(request)
	}
	pending, err := broker.PendingTVs(viewer)
	if err != nil || len(pending) != 8 {
		t.Fatalf("pending = %v, %v", pending, err)
	}
	encoded, _ := json.Marshal(pending)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "token") {
		t.Fatal("credentials exposed")
	}
	for _, secret := range secrets {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("polling secret exposed")
		}
	}
	if _, err := broker.PendingTVs(Viewer{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("anonymous read = %v", err)
	}
	if _, err := broker.PendingTVs(Viewer{ID: "bad\nviewer"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid viewer = %v", err)
	}
}

func TestPreviewDoesNotApproveAndExpiresWithRequest(t *testing.T) {
	t.Parallel()
	broker := New(time.Minute)
	now := time.Now()
	broker.now = func() time.Time { return now }
	secret, request, _ := broker.Create(Request{Device: "Living Room", Numeric: true})
	viewer := Viewer{ID: "viewer"}
	for _, code := range []string{"", "12345", "1234567", "12345x", " 123456", "１２３４５６", strings.Repeat("1", 1024)} {
		if _, err := broker.Preview(code, viewer); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid code %q: %v", code, err)
		}
	}
	preview, err := broker.Preview(request.Code, viewer)
	if err != nil || preview.Device != "Living Room" || preview.Code != request.Code || preview.ExpiresAt.Location() != time.UTC {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	if _, err := broker.Consume(secret); !errors.Is(err, ErrPending) {
		t.Fatalf("preview changed approval: %v", err)
	}
	now = now.Add(time.Minute)
	if _, err := broker.Preview(request.Code, viewer); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expired preview = %v", err)
	}
	if err := broker.Approve(request.Code, viewer); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expired approval = %v", err)
	}
}

func TestPreviewHidesRemoteAndApprovedRequests(t *testing.T) {
	t.Parallel()
	broker := New(time.Minute)
	viewer := Viewer{ID: "viewer"}
	_, remote, _ := broker.Create(Request{Numeric: true, Remote: true})
	if _, err := broker.Preview(remote.Code, viewer); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("remote preview = %v", err)
	}
	secret, local, _ := broker.Create(Request{Device: "Kinosail TV", Client: "Kinosail", Numeric: true})
	if err := broker.Approve(local.Code, viewer); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Preview(local.Code, viewer); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("approved preview = %v", err)
	}
	pending, _ := broker.PendingTVs(viewer)
	if len(pending) != 0 {
		t.Fatal("approved request still prompts")
	}
	if err := broker.Approve(local.Code, Viewer{ID: "other"}); !errors.Is(err, ErrAlreadyApproved) {
		t.Fatalf("profile replacement = %v", err)
	}
	grant, err := broker.Consume(secret)
	if err != nil || grant.ProfileID != viewer.ID {
		t.Fatalf("grant = %+v, %v", grant, err)
	}
	if _, err := broker.Consume(secret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replayed consume = %v", err)
	}
}

func TestApprovalReadHTTPBoundaries(t *testing.T) { //nolint:cyclop // The endpoint table compares authorized and rejected approval reads.
	t.Parallel()
	for _, path := range []string{"/api/v1/quick-connect/pending?unknown=1", "/api/v1/quick-connect/pending?"} {
		fixture := newApplicationFixture(t)
		recorder := httptest.NewRecorder()
		fixture.application.PendingTVsAPI(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest || fixture.currentCalls != 0 {
			t.Fatalf("invalid read = %d", recorder.Code)
		}
	}
	fixture := newApplicationFixture(t)
	secret, pending, _ := fixture.broker.Create(Request{Device: "Kinosail TV", Client: "Kinosail", Numeric: true})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/quick-connect/"+pending.Code, nil)
	request.SetPathValue("code", pending.Code)
	recorder := httptest.NewRecorder()
	fixture.application.PreviewAPI(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" || strings.Contains(recorder.Body.String(), secret) {
		t.Fatalf("preview = %d %s", recorder.Code, recorder.Body.String())
	}
	if _, err := fixture.broker.Consume(secret); !errors.Is(err, ErrPending) {
		t.Fatalf("read approved device: %v", err)
	}
	recorder = httptest.NewRecorder()
	serveRemote(fixture.application.PendingTVsAPI, recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/quick-connect/pending", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("public list = %d", recorder.Code)
	}
	for range 61 {
		recorder = httptest.NewRecorder()
		fixture.application.PendingTVsAPI(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/quick-connect/pending", nil))
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("unbounded list = %d", recorder.Code)
	}
}

func TestScannedLinkValidationAndExplicitConfirmation(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"", "?code=", "?code=12345", "?code=1234567", "?code=12345x", "?code=123456&code=654321", "?code=123456&unknown=1", "?code=%ZZ", "?code=" + strings.Repeat("1", 1000)} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/connect"+query, nil)
		if _, ok := ReadLinkCode(request); ok {
			t.Fatalf("accepted %q", query)
		}
	}
	fixture := newApplicationFixture(t)
	secret, request, _ := fixture.broker.Create(Request{Device: "Living Room", Numeric: true})
	fixture.application.Page(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/quick-connect?code="+request.Code, nil))
	if fixture.rendered.Code != request.Code || fixture.rendered.Device != "Living Room" || fixture.rendered.AutoSubmit {
		t.Fatalf("confirmation = %+v", fixture.rendered)
	}
	if _, err := fixture.broker.Consume(secret); !errors.Is(err, ErrPending) {
		t.Fatalf("GET approved request: %v", err)
	}
}

func TestCancelRequiresPollingSecretAndCannotAffectOtherRequests(t *testing.T) {
	t.Parallel()
	broker := New(time.Minute)
	secret, pending, _ := broker.Create(Request{Numeric: true})
	other, _, _ := broker.Create(Request{Numeric: true})
	for _, invalid := range []string{"", "bad\nsecret", strings.Repeat("x", 129)} {
		if err := broker.Cancel(invalid); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid cancel = %v", err)
		}
	}
	_ = broker.Cancel(pending.Code)
	if _, found := broker.Status(secret); !found {
		t.Fatal("public code cancelled request")
	}
	if err := broker.Cancel(secret); err != nil {
		t.Fatal(err)
	}
	if _, found := broker.Status(secret); found {
		t.Fatal("cancel did not remove request")
	}
	if _, found := broker.Status(other); !found {
		t.Fatal("cancel removed another request")
	}
	if err := broker.Cancel(secret); err != nil {
		t.Fatal("cancel is not idempotent")
	}
	if err := broker.Approve(pending.Code, Viewer{ID: "viewer"}); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("cancelled request approved: %v", err)
	}
}
