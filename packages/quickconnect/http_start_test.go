package quickconnect

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

type quickConnectHTTPResponse struct {
	Code, Secret, Token string
	Status              string
	ExpiresIn           int
}

func decodeQuickConnectResponse(t *testing.T, recorder *httptest.ResponseRecorder) quickConnectHTTPResponse {
	t.Helper()
	var response quickConnectHTTPResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response %d = %q: %v", recorder.Code, recorder.Body.String(), err)
	}
	return response
}

func serveRemote(handler http.HandlerFunc, recorder *httptest.ResponseRecorder, request *http.Request) {
	identitycore.Remote(handler).ServeHTTP(recorder, request)
}

func TestApplicationStartAcceptsPlayerTransports(t *testing.T) { //nolint:cyclop // The accepted transport matrix remains below the repository complexity limit.
	t.Parallel()
	tests := []struct{ name, contentType, body string }{
		{"empty", "", ""},
		{"JSON", "application/json; charset=utf-8", `{"Device":"TV"}`},
		{"form", "application/x-www-form-urlencoded", "device=Living+Room"},
		{"empty JSON", "application/json", `{}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newApplicationFixture(t)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			recorder := httptest.NewRecorder()
			fixture.application.Start(recorder, request)
			response := decodeQuickConnectResponse(t, recorder)
			if recorder.Code != http.StatusCreated || len(response.Code) != 6 || strings.Trim(response.Code, "0123456789") != "" || response.Secret == "" {
				t.Fatalf("start = %d %#v", recorder.Code, response)
			}
			connection, found := fixture.broker.Status(response.Secret)
			if !found || (test.name != "empty" && test.name != "empty JSON" && connection.Device == "") {
				t.Fatalf("connection = %#v, %v", connection, found)
			}
		})
	}
}

func TestApplicationStartMarksPublicRequest(t *testing.T) {
	t.Parallel()
	fixture := newApplicationFixture(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", nil)
	recorder := httptest.NewRecorder()
	serveRemote(fixture.application.Start, recorder, request)
	response := decodeQuickConnectResponse(t, recorder)
	if err := fixture.application.Approve(fixture.profiles["viewer"], response.Code, true); err != nil {
		t.Fatal(err)
	}
	token, _, err := fixture.application.Consume(response.Secret)
	if err != nil || fixture.sessions[identitycore.SessionKey(token)].Channel != "public" {
		t.Fatalf("public consume = %q, %v, %#v", token, err, fixture.sessions)
	}
}

type failingQuickConnectBody struct{}

func (failingQuickConnectBody) Read([]byte) (int, error) { return 0, errors.New("read") }
func (failingQuickConnectBody) Close() error             { return nil }

func TestApplicationStartRejectsInvalidInputBeforeCreation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, target, contentType, body string
		failing                         bool
	}{
		{"query", "/api/v1/quick-connect?device=TV", "application/json", `{"device":"TV"}`, false},
		{"bad media type", "/api/v1/quick-connect", "not a media type", "", false},
		{"JSONP", "/api/v1/quick-connect", "application/jsonp", `{"device":"TV"}`, false},
		{"malformed JSON", "/api/v1/quick-connect", "application/json", `{`, false},
		{"duplicate JSON", "/api/v1/quick-connect", "application/json", `{"device":"TV","device":"Other"}`, false},
		{"unknown JSON", "/api/v1/quick-connect", "application/json", `{"other":"TV"}`, false},
		{"nonstring JSON", "/api/v1/quick-connect", "application/json", `{"device":3}`, false},
		{"trailing JSON", "/api/v1/quick-connect", "application/json", `{"device":"TV"}{}`, false},
		{"nonempty raw", "/api/v1/quick-connect", "", "x", false},
		{"duplicate form", "/api/v1/quick-connect", "application/x-www-form-urlencoded", "device=TV&device=Other", false},
		{"unknown form", "/api/v1/quick-connect", "application/x-www-form-urlencoded", "device=TV&other=x", false},
		{"oversized", "/api/v1/quick-connect", "application/json", `{"device":"` + strings.Repeat("x", RequestBodyMaximum) + `"}`, false},
		{"read failure", "/api/v1/quick-connect", "", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newApplicationFixture(t)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.target, strings.NewReader(test.body))
			if test.failing {
				request.Body = failingQuickConnectBody{}
			}
			request.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()
			fixture.application.Start(recorder, request)
			if recorder.Code != http.StatusBadRequest || fixture.limits.Starts.TrackedClients() != 0 {
				t.Fatalf("invalid start = %d %q starts=%d", recorder.Code, recorder.Body.String(), fixture.limits.Starts.TrackedClients())
			}
		})
	}
}

func TestApplicationStartLimitsAndBrokerFailures(t *testing.T) { //nolint:cyclop,gocognit // One bounded matrix keeps request, start, and broker limits explicit.
	t.Parallel()
	t.Run("request limit", func(t *testing.T) {
		fixture := newApplicationFixture(t)
		var recorder *httptest.ResponseRecorder
		for range 61 {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", strings.NewReader("{"))
			request.Header.Set("Content-Type", "application/json")
			request.RemoteAddr = "192.0.2.1:10"
			recorder = httptest.NewRecorder()
			fixture.application.Start(recorder, request)
		}
		if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "60" || fixture.limits.Starts.TrackedClients() != 0 {
			t.Fatalf("request limit = %d %#v", recorder.Code, recorder.Header())
		}
	})
	t.Run("start limit", func(t *testing.T) {
		fixture := newApplicationFixture(t)
		var recorder *httptest.ResponseRecorder
		for range 21 {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", nil)
			request.RemoteAddr = "192.0.2.2:10"
			recorder = httptest.NewRecorder()
			fixture.application.Start(recorder, request)
		}
		if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "60" {
			t.Fatalf("start limit = %d %#v", recorder.Code, recorder.Header())
		}
	})
	for name, configure := range map[string]func(*applicationFixture){
		"capacity": func(fixture *applicationFixture) {
			for range maxPending {
				if _, _, err := fixture.broker.Create(Request{}); err != nil {
					t.Fatal(err)
				}
			}
		},
		"code failure": func(fixture *applicationFixture) {
			fixture.broker.code = func(bool) (string, error) { return "", errors.New("random") }
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newApplicationFixture(t)
			configure(fixture)
			recorder := httptest.NewRecorder()
			fixture.application.Start(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", nil))
			if recorder.Code != http.StatusTooManyRequests {
				t.Fatalf("broker failure = %d %q", recorder.Code, recorder.Body.String())
			}
		})
	}
	fixture := newApplicationFixture(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", strings.NewReader(`{"device":"Living\nRoom"}`))
	request.Header.Set("Content-Type", "application/json")
	fixture.application.Start(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid device = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestReadStringJSONRejectsMissingAndBrokenObjects(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"array": `[]`, "missing": `{}`, "broken close": `{"secret":"x"`, "trailing": `{"secret":"x"} true`,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
			if _, ok := readStringJSON(httptest.NewRecorder(), request, "secret", true); ok {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
}

var _ io.ReadCloser = failingQuickConnectBody{}
