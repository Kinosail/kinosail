package settings

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type errorRecord struct {
	message       string
	status, calls int
}

func (record *errorRecord) write(writer http.ResponseWriter, _ *http.Request, message string, status int) {
	record.message, record.status, record.calls = message, status, record.calls+1
	writer.WriteHeader(status)
}

func TestSavePlaybackResultPaths(t *testing.T) { //nolint:cyclop // One matrix proves successful and rejected HTTP side effects.
	t.Parallel()
	valid := url.Values{"mode": {"automatic"}, "subtitles": {"on"}, "autoplay": {"true"}, "markers": {"intro", "credits"}}
	for _, test := range []struct {
		name      string
		values    url.Values
		changeErr error
		wantCalls int
		wantCode  int
	}{
		{"malformed", url.Values{"mode": {"automatic"}, "unknown": {"x"}}, nil, 0, http.StatusBadRequest},
		{"autoplay", url.Values{"mode": {"automatic"}, "autoplay": {"false"}}, nil, 0, http.StatusBadRequest},
		{"change", valid, errors.New("change failed"), 1, http.StatusBadRequest},
		{"success", valid, nil, 1, http.StatusSeeOther},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder, failures, calls := httptest.NewRecorder(), &errorRecord{}, 0
			SavePlayback(func(input Playback) error {
				calls++
				if input.PlaybackMode != "automatic" || input.Subtitles != "on" || !input.Autoplay || len(input.AutoSkip) != 2 {
					t.Fatalf("input = %#v", input)
				}
				return test.changeErr
			}, failures.write)(recorder, formRequest("/settings/playback", test.values))
			if recorder.Code != test.wantCode || calls != test.wantCalls || failures.calls != boolInt(test.wantCode == http.StatusBadRequest) || test.wantCode == http.StatusSeeOther && recorder.Header().Get("Location") != "/settings" {
				t.Fatalf("code=%d calls=%d failures=%#v location=%q", recorder.Code, calls, failures, recorder.Header().Get("Location"))
			}
		})
	}
}

func TestSaveOnboardingAPIResultPaths(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                 string
		read, enabled        bool
		changeErr            error
		wantCode, changes    int
		wantError, wantWrite bool
	}{
		{"decode", false, false, nil, http.StatusOK, 0, false, false},
		{"missing", true, false, nil, http.StatusBadRequest, 0, true, false},
		{"change", true, true, errors.New("save failed"), http.StatusInternalServerError, 1, true, false},
		{"success", true, true, nil, http.StatusOK, 1, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder, changes, wrote, failed := httptest.NewRecorder(), 0, false, false
			SaveOnboardingAPI(func(enabled bool) error { changes++; return test.changeErr }, func(_ http.ResponseWriter, _ *http.Request, target any) bool {
				if test.read && test.name != "missing" {
					enabled := test.enabled
					target.(*struct {
						Enabled *bool `json:"enabled"`
					}).Enabled = &enabled
				}
				return test.read
			}, func(_ http.ResponseWriter, value any, status int) {
				wrote = status == http.StatusOK && value.(map[string]string)["status"] == "saved"
			}, func(writer http.ResponseWriter, _ error, status int) {
				failed = true
				writer.WriteHeader(status)
			})(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", nil))
			if recorder.Code != test.wantCode || changes != test.changes || failed != test.wantError || wrote != test.wantWrite {
				t.Fatalf("code=%d changes=%d failed=%t wrote=%t", recorder.Code, changes, failed, wrote)
			}
		})
	}
}

func TestSaveScanFrequencyResultPaths(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                               string
		values                             url.Values
		changeErr                          error
		wantCode, wantChanges, wantApplies int
	}{
		{"malformed", url.Values{"frequency": {"5m", "1h"}}, nil, 400, 0, 0},
		{"change", url.Values{"frequency": {"5m"}}, errors.New("change failed"), 400, 1, 0},
		{"success", url.Values{"frequency": {"5m"}}, nil, 303, 1, 1},
	} {
		recorder, failures, changes, applies := httptest.NewRecorder(), &errorRecord{}, 0, 0
		SaveScanFrequency(func(value string) error {
			changes++
			if value != "5m" {
				t.Fatalf("frequency = %q", value)
			}
			return test.changeErr
		}, func(value string) {
			applies++
			if value != "5m" {
				t.Fatalf("applied = %q", value)
			}
		}, "/return", failures.write)(recorder, formRequest("/settings/scans", test.values))
		if recorder.Code != test.wantCode || changes != test.wantChanges || applies != test.wantApplies || test.wantCode == 303 && recorder.Header().Get("Location") != "/return" {
			t.Fatalf("%s code=%d changes=%d applies=%d", test.name, recorder.Code, changes, applies)
		}
	}
}

func TestCreateAPIKeyResultPaths(t *testing.T) { //nolint:cyclop,gocognit // This covers the strict form, operation, and rendering boundaries.
	t.Parallel()
	valid := url.Values{"name": {"Remote"}, "scopes": {"library"}, "stream": {"stream"}}
	for _, test := range []struct {
		name                       string
		values                     url.Values
		createErr, renderErr       error
		wantCode, creates, renders int
	}{
		{"malformed", url.Values{"name": {"Remote"}, "unknown": {"x"}}, nil, nil, 400, 0, 0},
		{"missing name", url.Values{"scopes": {"library"}}, nil, nil, 400, 0, 0},
		{"large name", url.Values{"name": {strings.Repeat("n", 81)}, "scopes": {"library"}}, nil, nil, 400, 0, 0},
		{"no scopes", url.Values{"name": {"Remote"}}, nil, nil, 400, 0, 0},
		{"wrong scope", url.Values{"name": {"Remote"}, "scopes": {"other"}}, nil, nil, 400, 0, 0},
		{"wrong optional", url.Values{"name": {"Remote"}, "admin": {"true"}}, nil, nil, 400, 0, 0},
		{"boundary name", url.Values{"name": {strings.Repeat("n", 80)}, "scopes": {"library"}}, nil, nil, 201, 1, 1},
		{"operation", valid, errors.New("create failed"), nil, 400, 1, 0},
		{"render", valid, nil, errors.New("render failed"), 201, 1, 1},
		{"success", valid, nil, nil, 201, 1, 1},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			recorder, failures, creates, renders := httptest.NewRecorder(), &errorRecord{}, 0, 0
			CreateAPIKey(func(request *http.Request, name, scopes string) (string, error) {
				creates++
				expectedScopes := test.values.Get("scopes") + "," + test.values.Get("write") + "," + test.values.Get("stream") + "," + test.values.Get("download") + "," + test.values.Get("admin")
				if request.URL.Path != "/settings/api-keys" || name != test.values.Get("name") || scopes != expectedScopes {
					t.Fatalf("create input = %q %q %q", request.URL.Path, name, scopes)
				}
				return "secret", test.createErr
			}, func(_ http.ResponseWriter, _ *http.Request, name, secret string) error {
				renders++
				if name != test.values.Get("name") || secret != "secret" {
					t.Fatalf("render = %q %q", name, secret)
				}
				return test.renderErr
			}, failures.write)(recorder, formRequest("/settings/api-keys", test.values))
			if recorder.Code != test.wantCode || creates != test.creates || renders != test.renders || (test.name == "render") != (failures.message == "API key view failed") {
				t.Fatalf("code=%d creates=%d renders=%d failures=%#v", recorder.Code, creates, renders, failures)
			}
			if test.name == "success" && (recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" || recorder.Header().Get("Cache-Control") != "no-store") {
				t.Fatalf("headers = %v", recorder.Header())
			}
		})
	}
}

func TestDownloadBackupResultPaths(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name            string
		available       bool
		archiveErr      error
		wantCode, calls int
	}{{"unavailable", false, nil, 503, 0}, {"failure", true, errors.New("archive"), 503, 1}, {"success", true, nil, 200, 1}} {
		recorder, failures, calls := httptest.NewRecorder(), &errorRecord{}, 0
		DownloadBackup(test.available, func(writer io.Writer) error { calls++; _, _ = writer.Write([]byte("archive")); return test.archiveErr }, failures.write)(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/settings/backup", nil))
		if recorder.Code != test.wantCode || calls != test.calls {
			t.Fatalf("%s code=%d calls=%d", test.name, recorder.Code, calls)
		}
		if test.name == "success" && (recorder.Body.String() != "archive" || recorder.Header().Get("Content-Type") != "application/gzip" || recorder.Header().Get("Content-Disposition") != `attachment; filename="kinosail-backup.tar.gz"`) {
			t.Fatalf("response = %q %v", recorder.Body.String(), recorder.Header())
		}
	}
}

func TestBackupPageAndVerificationHandlers(t *testing.T) { //nolint:cyclop,gocognit // One lifecycle keeps the paired backup handlers and failures together.
	t.Parallel()
	t.Run("page", func(t *testing.T) {
		for _, renderErr := range []error{nil, errors.New("render failed")} {
			recorder, failures := httptest.NewRecorder(), &errorRecord{}
			ShowBackups(func() string { return "status" }, func(_ http.ResponseWriter, request *http.Request, value any) error {
				if request.URL.Path != "/settings/backups" || value != "status" {
					t.Fatalf("render input = %q %#v", request.URL.Path, value)
				}
				return renderErr
			}, failures.write)(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/backups", nil))
			if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" || failures.calls != boolInt(renderErr != nil) {
				t.Fatalf("headers=%v failures=%#v", recorder.Header(), failures)
			}
		}
	})
	t.Run("verify", func(t *testing.T) {
		for _, verifyErr := range []error{nil, errors.New("verify failed")} {
			recorder, failures := httptest.NewRecorder(), &errorRecord{}
			VerifyBackup(func() error { return verifyErr }, failures.write)(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/backups/verify", nil))
			if verifyErr == nil && (recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/settings/backups") {
				t.Fatalf("success = %d %q", recorder.Code, recorder.Header().Get("Location"))
			}
			if verifyErr != nil && (recorder.Code != http.StatusServiceUnavailable || failures.message != verifyErr.Error()) {
				t.Fatalf("failure = %d %#v", recorder.Code, failures)
			}
		}
	})
}

func TestDecodeFormEnvelope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, contentType string
		values            url.Values
		query             string
		maximum           int64
	}{
		{"valid parameter", "application/x-www-form-urlencoded; charset=UTF-8", url.Values{"key": {"one"}}, "", 128},
		{"bad media", "text/plain", url.Values{"key": {"one"}}, "", 128},
		{"bad media syntax", "%%", url.Values{"key": {"one"}}, "", 128},
		{"query", "application/x-www-form-urlencoded", url.Values{"key": {"one"}}, "x=1", 128},
		{"unknown", "application/x-www-form-urlencoded", url.Values{"other": {"one"}}, "", 128},
		{"too many", "application/x-www-form-urlencoded", url.Values{"key": {"one", "two"}}, "", 128},
		{"too large", "application/x-www-form-urlencoded", url.Values{"key": {strings.Repeat("x", 129)}}, "", 16},
	}
	for _, test := range tests {
		request := formRequest("/settings", test.values)
		request.Header.Set("Content-Type", test.contentType)
		request.URL.RawQuery = test.query
		err := decodeForm(httptest.NewRecorder(), request, test.maximum, map[string]int{"key": 1})
		if (test.name == "valid parameter") != (err == nil) {
			t.Fatalf("%s err=%v", test.name, err)
		}
	}
}

func formRequest(path string, values url.Values) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
