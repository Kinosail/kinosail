package federationconfig

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseOIDCConfigurationForm(t *testing.T) { //nolint:cyclop // The matrix covers each strict form field and envelope.
	t.Parallel()
	valid := url.Values{
		"key": {OIDCConfigurationKey}, "issuer": {"https://identity.example"}, "clientId": {"client"},
		"clientSecret": {"secret"}, "redirectUrl": {"https://media.example/login/oidc/callback"}, "identityClaim": {"sub"},
	}
	config, err := ParseOIDCConfigurationForm(httptest.NewRecorder(), oidcFormRequest(valid))
	if err != nil || config.Issuer != valid.Get("issuer") || config.ClientID != "client" || config.ClientSecret != "secret" || config.RedirectURL != valid.Get("redirectUrl") || config.IdentityClaim != "sub" {
		t.Fatalf("valid form = %#v %v", config, err)
	}
	withoutSecret := cloneOIDCFormValues(valid)
	withoutSecret.Set("clientSecret", "")
	if config, parseErr := ParseOIDCConfigurationForm(httptest.NewRecorder(), oidcFormRequest(withoutSecret)); parseErr != nil || config.ClientSecret != "" {
		t.Fatalf("optional secret = %#v %v", config, parseErr)
	}
	cases := map[string]func(url.Values, *http.Request){
		"wrong key":          func(values url.Values, _ *http.Request) { values.Set("key", "other") },
		"missing issuer":     func(values url.Values, _ *http.Request) { values.Del("issuer") },
		"empty issuer":       func(values url.Values, _ *http.Request) { values.Set("issuer", "") },
		"oversized issuer":   func(values url.Values, _ *http.Request) { values.Set("issuer", strings.Repeat("x", 2049)) },
		"missing client":     func(values url.Values, _ *http.Request) { values.Del("clientId") },
		"oversized client":   func(values url.Values, _ *http.Request) { values.Set("clientId", strings.Repeat("x", 513)) },
		"missing secret":     func(values url.Values, _ *http.Request) { values.Del("clientSecret") },
		"oversized secret":   func(values url.Values, _ *http.Request) { values.Set("clientSecret", strings.Repeat("x", 4097)) },
		"missing redirect":   func(values url.Values, _ *http.Request) { values.Del("redirectUrl") },
		"oversized redirect": func(values url.Values, _ *http.Request) { values.Set("redirectUrl", strings.Repeat("x", 2049)) },
		"missing claim":      func(values url.Values, _ *http.Request) { values.Del("identityClaim") },
		"oversized claim":    func(values url.Values, _ *http.Request) { values.Set("identityClaim", strings.Repeat("x", 257)) },
		"repeated value":     func(values url.Values, _ *http.Request) { values["issuer"] = []string{"one", "two"} },
		"unknown value":      func(values url.Values, _ *http.Request) { values.Set("unknown", "value") },
		"query":              func(_ url.Values, request *http.Request) { request.URL.RawQuery = "extra=true" },
		"media type":         func(_ url.Values, request *http.Request) { request.Header.Set("Content-Type", "text/plain") },
	}
	for name, mutate := range cases {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			values := cloneOIDCFormValues(valid)
			request := oidcFormRequest(values)
			mutate(values, request)
			if request.Header.Get("Content-Type") != "text/plain" && request.URL.RawQuery == "" {
				request = oidcFormRequest(values)
			}
			if _, parseErr := ParseOIDCConfigurationForm(httptest.NewRecorder(), request); parseErr == nil {
				t.Fatal("invalid form was accepted")
			}
		})
	}
}

func TestOIDCFormFieldAcceptsExactMaximum(t *testing.T) {
	t.Parallel()
	request := &http.Request{PostForm: url.Values{"field": {"x"}}}
	if value, ok := oidcFormField(request, "field", 1, true); !ok || value != "x" {
		t.Fatalf("boundary = %q %t", value, ok)
	}
}

func TestSaveOIDCConfigurationResultPaths(t *testing.T) {
	t.Parallel()
	valid := url.Values{
		"key": {OIDCConfigurationKey}, "issuer": {"https://identity.example"}, "clientId": {"client"},
		"clientSecret": {"secret"}, "redirectUrl": {"https://media.example/login/oidc/callback"}, "identityClaim": {"sub"},
	}
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
			SaveOIDCConfiguration(recorder, oidcFormRequest(test.values), func(string, string, string, string, string, bool) error { return test.changeErr }, func(_ http.ResponseWriter, _ *http.Request, _ string, status int) {
				errorsWritten++
				recorder.WriteHeader(status)
			})
			if recorder.Code != test.wantStatus || errorsWritten != test.wantErrors || test.wantStatus == http.StatusSeeOther && recorder.Header().Get("Location") != "/settings/configuration#integrations.oidc" {
				t.Fatalf("result = %d errors=%d location=%q", recorder.Code, errorsWritten, recorder.Header().Get("Location"))
			}
		})
	}
}

func oidcFormRequest(values url.Values) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://media.example/settings/configuration", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func cloneOIDCFormValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, value := range values {
		clone[key] = append([]string(nil), value...)
	}
	return clone
}
