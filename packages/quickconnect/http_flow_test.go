package quickconnect

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func TestApplicationPollStatesAndTransports(t *testing.T) { //nolint:cyclop // The transport matrix remains below the repository complexity limit.
	t.Parallel()
	t.Run("pending", func(t *testing.T) {
		fixture := newApplicationFixture(t)
		secret, _, err := fixture.broker.Create(Request{})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/token", strings.NewReader("secret="+secret))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		fixture.application.Poll(recorder, request)
		response := decodeQuickConnectResponse(t, recorder)
		if recorder.Code != http.StatusAccepted || response.Status != "pending" {
			t.Fatalf("pending = %d %#v", recorder.Code, response)
		}
	})
	t.Run("local form", func(t *testing.T) {
		fixture := newApplicationFixture(t)
		secret := fixture.approved(t, Request{Device: "TV"}, fixture.profiles["viewer"])
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/token", strings.NewReader("secret="+secret))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		fixture.application.Poll(recorder, request)
		response := decodeQuickConnectResponse(t, recorder)
		if recorder.Code != http.StatusCreated || response.Token == "" || response.ExpiresIn != 2592000 {
			t.Fatalf("local poll = %d %#v", recorder.Code, response)
		}
	})
	t.Run("public JSON", func(t *testing.T) {
		fixture := newApplicationFixture(t)
		secret := fixture.approved(t, Request{Remote: true}, fixture.profiles["viewer"])
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/token", strings.NewReader(`{"Secret":"`+secret+`"}`))
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		recorder := httptest.NewRecorder()
		serveRemote(fixture.application.Poll, recorder, request)
		response := decodeQuickConnectResponse(t, recorder)
		if recorder.Code != http.StatusCreated || response.Token == "" || response.ExpiresIn != 28800 {
			t.Fatalf("public poll = %d %#v", recorder.Code, response)
		}
	})
	t.Run("missing", func(t *testing.T) {
		fixture := newApplicationFixture(t)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/token", strings.NewReader("secret=missing"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		fixture.application.Poll(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("missing poll = %d %q", recorder.Code, recorder.Body.String())
		}
	})
}

func TestApplicationPollRejectsInvalidInputWithoutConsumption(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, target, contentType, body string }{
		{"query", "/api/v1/quick-connect/token?x=1", "application/x-www-form-urlencoded", "secret=x"},
		{"missing media", "/api/v1/quick-connect/token", "", "secret=x"},
		{"JSONP", "/api/v1/quick-connect/token", "application/jsonp", `{"secret":"x"}`},
		{"missing JSON", "/api/v1/quick-connect/token", "application/json", `{}`},
		{"duplicate form", "/api/v1/quick-connect/token", "application/x-www-form-urlencoded", "secret=x&secret=x"},
		{"unknown form", "/api/v1/quick-connect/token", "application/x-www-form-urlencoded", "secret=x&other=y"},
		{"oversized", "/api/v1/quick-connect/token", "application/json", `{"secret":"` + strings.Repeat("x", RequestBodyMaximum) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newApplicationFixture(t)
			secret := fixture.approved(t, Request{}, fixture.profiles["viewer"])
			body := strings.ReplaceAll(test.body, "secret=x", "secret="+secret)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.target, strings.NewReader(body))
			request.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()
			fixture.application.Poll(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("invalid poll = %d %q", recorder.Code, recorder.Body.String())
			}
			if _, found := fixture.broker.Status(secret); !found {
				t.Fatal("invalid poll consumed request")
			}
		})
	}
	fixture := newApplicationFixture(t)
	var recorder *httptest.ResponseRecorder
	for range 121 {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/token", strings.NewReader("{"))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "192.0.2.3:10"
		recorder = httptest.NewRecorder()
		fixture.application.Poll(recorder, request)
	}
	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "60" {
		t.Fatalf("poll limit = %d %#v", recorder.Code, recorder.Header())
	}
}

func TestApplicationPage(t *testing.T) {
	t.Parallel()
	fixture := newApplicationFixture(t)
	fixture.renderErr = errors.New("ignored")
	recorder := httptest.NewRecorder()
	fixture.application.Page(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/quick-connect", nil))
	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" || fixture.renders != 1 || !fixture.rendered.AutoSubmit {
		t.Fatalf("page = %#v headers=%#v", fixture.rendered, recorder.Header())
	}
}

func TestApplicationApprovePageBoundaries(t *testing.T) { //nolint:cyclop,gocognit // The approval matrix keeps every no-side-effect boundary explicit.
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		fixture := newApplicationFixture(t)
		secret, connection, err := fixture.broker.Create(Request{})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/quick-connect", strings.NewReader("code="+connection.Code))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		fixture.application.ApprovePage(recorder, request)
		if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/" || fixture.currentCalls != 1 || fixture.recentCalls != 1 {
			t.Fatalf("approval = %d %#v", recorder.Code, recorder.Header())
		}
		if state, found := fixture.broker.Status(secret); !found || !state.Approved {
			t.Fatalf("state = %#v, %v", state, found)
		}
	})
	for name, test := range map[string]struct{ target, body string }{
		"query":        {"/quick-connect?x=1", "code=123456"},
		"duplicate":    {"/quick-connect", "code=123456&code=123456"},
		"unknown":      {"/quick-connect", "code=123456&other=x"},
		"oversized":    {"/quick-connect", "code=" + strings.Repeat("x", RequestBodyMaximum+1)},
		"long code":    {"/quick-connect", "code=" + strings.Repeat("x", 17)},
		"missing code": {"/quick-connect", ""},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newApplicationFixture(t)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.target, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			fixture.application.ApprovePage(httptest.NewRecorder(), request)
			if fixture.errors != 1 || fixture.errorStatus != http.StatusBadRequest || fixture.currentCalls != 0 || fixture.recentCalls != 0 {
				t.Fatalf("invalid approval effects = %#v", fixture)
			}
		})
	}
	fixture := newApplicationFixture(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/quick-connect", nil)
	serveRemote(fixture.application.ApprovePage, httptest.NewRecorder(), request)
	if fixture.errorStatus != http.StatusNotFound || fixture.currentCalls != 0 {
		t.Fatalf("public web approval = %#v", fixture)
	}
	fixture = newApplicationFixture(t)
	fixture.current = identitycore.Profile{ID: "owner", Owner: true}
	_, connection, _ := fixture.broker.Create(Request{Remote: true})
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/quick-connect", strings.NewReader("code="+connection.Code))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	fixture.application.ApprovePage(httptest.NewRecorder(), request)
	if fixture.errorStatus != http.StatusForbidden {
		t.Fatalf("remote owner status = %d", fixture.errorStatus)
	}
}

func TestApplicationApproveAPIBoundaries(t *testing.T) { //nolint:cyclop // The API boundary matrix remains below the repository complexity limit.
	t.Parallel()
	fixture := newApplicationFixture(t)
	_, connection, _ := fixture.broker.Create(Request{})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/"+connection.Code, nil)
	request.SetPathValue("code", connection.Code)
	recorder := httptest.NewRecorder()
	fixture.application.ApproveAPI(recorder, request)
	if recorder.Code != http.StatusNoContent || fixture.currentCalls != 1 {
		t.Fatalf("API approval = %d %q", recorder.Code, recorder.Body.String())
	}
	for name, test := range map[string]struct{ target, body string }{
		"query": {"/api/v1/quick-connect/123456?x=1", ""},
		"body":  {"/api/v1/quick-connect/123456", "x"},
		"code":  {"/api/v1/quick-connect/" + strings.Repeat("x", 17), ""},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newApplicationFixture(t)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.target, strings.NewReader(test.body))
			if name == "code" {
				request.SetPathValue("code", strings.Repeat("x", 17))
			}
			fixture.application.ApproveAPI(recorder, request)
			if recorder.Code != http.StatusBadRequest || fixture.currentCalls != 0 || fixture.recentCalls != 0 {
				t.Fatalf("invalid API approval = %d calls=%d/%d", recorder.Code, fixture.currentCalls, fixture.recentCalls)
			}
		})
	}
	fixture = newApplicationFixture(t)
	recorder = httptest.NewRecorder()
	serveRemote(fixture.application.ApproveAPI, recorder, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/123456", nil))
	if recorder.Code != http.StatusNotFound || fixture.currentCalls != 0 {
		t.Fatalf("public API approval = %d calls=%d", recorder.Code, fixture.currentCalls)
	}
	fixture = newApplicationFixture(t)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/missing", nil)
	request.SetPathValue("code", "missing")
	recorder = httptest.NewRecorder()
	fixture.application.ApproveAPI(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing API approval = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestQuickConnectHTMLProfiles(t *testing.T) {
	t.Parallel()
	view := template.Must(template.New("quick").Parse(HTML))
	for _, test := range []struct {
		page Page
		want []string
	}{
		{PlayerPage(), []string{"Kinosail Player", `/static/quick-connect.js?v=1`, `data-quick-connect-digit`, "six-digit code"}},
		{SubtitlesPage(), []string{"Kinosail Subtitles", `maxlength="6"`, `inputmode="numeric"`, `pattern="[0-9]{6}"`}},
	} {
		var output bytes.Buffer
		if err := view.Execute(&output, test.page); err != nil {
			t.Fatal(err)
		}
		for _, fragment := range test.want {
			if !strings.Contains(output.String(), fragment) {
				t.Errorf("%s output missing %q", test.page.Product, fragment)
			}
		}
	}
}

func TestResponseJSONShape(t *testing.T) {
	t.Parallel()
	fixture := newApplicationFixture(t)
	recorder := httptest.NewRecorder()
	fixture.application.Start(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", nil))
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || len(body) != 2 {
		t.Fatalf("JSON shape = %#v, %v", body, err)
	}
}
