package scimapp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseConfigurationForm(t *testing.T) { //nolint:cyclop // The matrix covers each strict form field and envelope.
	t.Parallel()
	valid := url.Values{"key": {ConfigurationKey}, "token": {"token"}, "tokenExpiresOn": {"2026-09-06"}}
	input, err := ParseConfigurationForm(httptest.NewRecorder(), scimFormRequest(valid))
	if err != nil || input.Token != "token" || input.Expiration != "2026-09-06T23:59:59Z" {
		t.Fatalf("valid form = %#v %v", input, err)
	}
	cases := map[string]func(url.Values, *http.Request){
		"wrong key":       func(values url.Values, _ *http.Request) { values.Set("key", "other") },
		"missing token":   func(values url.Values, _ *http.Request) { values.Del("token") },
		"oversized token": func(values url.Values, _ *http.Request) { values.Set("token", strings.Repeat("x", 257)) },
		"missing date":    func(values url.Values, _ *http.Request) { values.Del("tokenExpiresOn") },
		"short date":      func(values url.Values, _ *http.Request) { values.Set("tokenExpiresOn", "2026-9-6") },
		"invalid date":    func(values url.Values, _ *http.Request) { values.Set("tokenExpiresOn", "2026-99-99") },
		"repeated value": func(values url.Values, _ *http.Request) {
			values["token"] = []string{"one", "two"}
		},
		"unknown value": func(values url.Values, _ *http.Request) { values.Set("unknown", "value") },
		"query":         func(_ url.Values, request *http.Request) { request.URL.RawQuery = "extra=true" },
		"media type": func(_ url.Values, request *http.Request) {
			request.Header.Set("Content-Type", "text/plain")
		},
	}
	for name, mutate := range cases {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			values := cloneFormValues(valid)
			request := scimFormRequest(values)
			mutate(values, request)
			if request.Body != nil && request.Header.Get("Content-Type") != "text/plain" && request.URL.RawQuery == "" {
				request = scimFormRequest(values)
			}
			if _, parseErr := ParseConfigurationForm(httptest.NewRecorder(), request); parseErr == nil {
				t.Fatal("invalid form was accepted")
			}
		})
	}
}

func TestSaveConfigurationResultPaths(t *testing.T) {
	t.Parallel()
	valid := url.Values{"key": {ConfigurationKey}, "token": {"token"}, "tokenExpiresOn": {"2026-09-06"}}
	tests := []struct {
		name       string
		values     url.Values
		changeErr  error
		wantStatus int
		wantErrors int
	}{
		{"invalid", url.Values{}, nil, http.StatusBadRequest, 1},
		{"change", valid, errors.New("change failed"), http.StatusConflict, 1},
		{"success", valid, nil, http.StatusSeeOther, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder, errorsWritten := httptest.NewRecorder(), 0
			SaveConfiguration(recorder, scimFormRequest(test.values), func(string, string, bool) error { return test.changeErr }, func(_ http.ResponseWriter, _ *http.Request, _ string, status int) {
				errorsWritten++
				recorder.WriteHeader(status)
			})
			if recorder.Code != test.wantStatus || errorsWritten != test.wantErrors || test.wantStatus == http.StatusSeeOther && recorder.Header().Get("Location") != "/settings/configuration#integrations.scim" {
				t.Fatalf("result = %d errors=%d location=%q", recorder.Code, errorsWritten, recorder.Header().Get("Location"))
			}
		})
	}
}

func scimFormRequest(values url.Values) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://media.example/settings/configuration", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func cloneFormValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, value := range values {
		clone[key] = append([]string(nil), value...)
	}
	return clone
}
