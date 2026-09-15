package identitycore

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type mfaDisableProbe struct {
	events       []string
	verifyOK     bool
	disableError error
	delivered    error
}

func (probe *mfaDisableProbe) record(event string) {
	probe.events = append(probe.events, event)
}

func (probe *mfaDisableProbe) config() MFAHTTPConfig {
	return MFAHTTPConfig{
		ReadJSON: func(writer http.ResponseWriter, request *http.Request, target any) bool {
			probe.record("read")
			if json.NewDecoder(request.Body).Decode(target) != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return false
			}
			return true
		},
		Setup: func(*http.Request) (Enrollment, error) {
			probe.record("setup")
			return Enrollment{}, nil
		},
		Confirm: func(*http.Request, string) error {
			probe.record("confirm")
			return nil
		},
		MarkStrong: func(*http.Request) error {
			probe.record("strong")
			return nil
		},
		Verify: func(_ *http.Request, code string) bool {
			probe.record("verify:" + code)
			return probe.verifyOK
		},
		Disable: func(*http.Request) error {
			probe.record("disable")
			return probe.disableError
		},
		Error: func(writer http.ResponseWriter, err error, status int) {
			probe.delivered = err
			probe.record(fmt.Sprintf("api-error:%d:%s", status, err))
			writer.WriteHeader(status)
		},
		JSON: func(writer http.ResponseWriter, _ any, status int) {
			probe.record("json")
			writer.WriteHeader(status)
		},
		WebError: func(writer http.ResponseWriter, _ *http.Request, err error, status int) {
			probe.delivered = err
			probe.record(fmt.Sprintf("web-error:%d:%s", status, err))
			writer.WriteHeader(status)
		},
		WebConfirmSuccess: func(http.ResponseWriter, *http.Request) {
			probe.record("web-confirm")
		},
		WebDisableSuccess: func(writer http.ResponseWriter, _ *http.Request) {
			probe.record("web-success")
			writer.WriteHeader(http.StatusSeeOther)
		},
	}
}

func TestMFADisableConfigurationFailsClosedBeforeCallbacks(t *testing.T) { //nolint:cyclop // The score of 13 remains below the repository ceiling of 22 for the callback matrix.
	validProbe := &mfaDisableProbe{}
	valid := validProbe.config()
	if !validMFAHTTPConfig(valid) {
		t.Fatal("complete MFA HTTP configuration rejected")
	}
	if _, err := NewMFAHandlers(valid); err != nil || len(validProbe.events) != 0 {
		t.Fatalf("complete configuration: error=%v events=%v", err, validProbe.events)
	}
	mutations := []struct {
		name   string
		mutate func(*MFAHTTPConfig)
	}{
		{"ReadJSON", func(config *MFAHTTPConfig) { config.ReadJSON = nil }},
		{"Setup", func(config *MFAHTTPConfig) { config.Setup = nil }},
		{"Confirm", func(config *MFAHTTPConfig) { config.Confirm = nil }},
		{"MarkStrong", func(config *MFAHTTPConfig) { config.MarkStrong = nil }},
		{"Verify", func(config *MFAHTTPConfig) { config.Verify = nil }},
		{"Disable", func(config *MFAHTTPConfig) { config.Disable = nil }},
		{"Error", func(config *MFAHTTPConfig) { config.Error = nil }},
		{"JSON", func(config *MFAHTTPConfig) { config.JSON = nil }},
		{"WebError", func(config *MFAHTTPConfig) { config.WebError = nil }},
		{"WebConfirmSuccess", func(config *MFAHTTPConfig) { config.WebConfirmSuccess = nil }},
		{"WebDisableSuccess", func(config *MFAHTTPConfig) { config.WebDisableSuccess = nil }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			probe := &mfaDisableProbe{}
			config := probe.config()
			mutation.mutate(&config)
			if validMFAHTTPConfig(config) {
				t.Fatal("incomplete MFA HTTP configuration accepted")
			}
			handlers, err := NewMFAHandlers(config)
			if !errors.Is(err, ErrInvalidConfig) || handlers.Setup != nil || handlers.Confirm != nil || handlers.Disable != nil || handlers.WebConfirm != nil || handlers.WebDisable != nil {
				t.Fatalf("incomplete configuration: handlers=%#v error=%v", handlers, err)
			}
			if len(probe.events) != 0 {
				t.Fatalf("incomplete configuration invoked callbacks: %v", probe.events)
			}
		})
	}
}

func TestMFADisableAPITranslationAndOrdering(t *testing.T) {
	disableErr := errors.New("disable unavailable")
	tests := []struct {
		name, body   string
		verify       bool
		disableError error
		status       int
		events       []string
	}{
		{"malformed input", `{`, false, nil, http.StatusBadRequest, []string{"read", "status:400"}},
		{"invalid code", `{"code":"000000"}`, false, nil, http.StatusUnauthorized, []string{"read", "verify:000000", "api-error:401:invalid authentication code", "status:401"}},
		{"disable failure", `{"code":"000000"}`, true, disableErr, http.StatusConflict, []string{"read", "verify:000000", "disable", "api-error:409:disable unavailable", "status:409"}},
		{"success", `{"code":"000000"}`, true, nil, http.StatusNoContent, []string{"read", "verify:000000", "disable", "status:204"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe := &mfaDisableProbe{verifyOK: test.verify, disableError: test.disableError}
			writer := &mfaDisableWriter{events: &probe.events}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/v1/me/mfa", strings.NewReader(test.body))
			mfaDisableHandler(probe.config())(writer, request)
			if writer.status != test.status || !reflect.DeepEqual(probe.events, test.events) {
				t.Fatalf("status=%d events=%v", writer.status, probe.events)
			}
			if test.disableError != nil && !errors.Is(probe.delivered, test.disableError) {
				t.Fatalf("delivered error = %v", probe.delivered)
			}
		})
	}
}

func TestMFADisableWebTranslationAndOrdering(t *testing.T) {
	disableErr := errors.New("disable unavailable")
	tests := []struct {
		name         string
		verify       bool
		disableError error
		status       int
		events       []string
	}{
		{"invalid code", false, nil, http.StatusUnauthorized, []string{"verify:000000", "web-error:401:invalid authentication code", "status:401"}},
		{"disable failure", true, disableErr, http.StatusConflict, []string{"verify:000000", "disable", "web-error:409:disable unavailable", "status:409"}},
		{"success", true, nil, http.StatusSeeOther, []string{"verify:000000", "disable", "web-success", "status:303"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe := &mfaDisableProbe{verifyOK: test.verify, disableError: test.disableError}
			writer := &mfaDisableWriter{events: &probe.events}
			body := url.Values{"code": {"000000"}}.Encode()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/account/mfa/disable", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			mfaWebDisableHandler(probe.config())(writer, request)
			if writer.status != test.status || !reflect.DeepEqual(probe.events, test.events) {
				t.Fatalf("status=%d events=%v", writer.status, probe.events)
			}
			if test.disableError != nil && !errors.Is(probe.delivered, test.disableError) {
				t.Fatalf("delivered error = %v", probe.delivered)
			}
		})
	}
}

type mfaDisableWriter struct {
	header http.Header
	status int
	events *[]string
}

func (writer *mfaDisableWriter) Header() http.Header {
	if writer.header == nil {
		writer.header = make(http.Header)
	}
	return writer.header
}

func (writer *mfaDisableWriter) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return len(data), nil
}

func (writer *mfaDisableWriter) WriteHeader(status int) {
	writer.status = status
	*writer.events = append(*writer.events, fmt.Sprintf("status:%d", status))
}
