package httpguard

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDecodeJSONAcceptsOneBoundedDocument(t *testing.T) {
	t.Parallel()
	var decoded map[string]bool
	if err := DecodeJSON(strings.NewReader(`{"ok":true}`), 64, &decoded, true); err != nil || !decoded["ok"] {
		t.Fatalf("decode = %#v, %v", decoded, err)
	}
}

func TestDecodeRequestJSONAcceptsOneStrictObject(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"ok":true}`))
	var target struct {
		OK bool `json:"ok"`
	}
	if err := DecodeRequestJSON(httptest.NewRecorder(), request, &target); err != nil || !target.OK {
		t.Fatalf("request JSON = %#v, %v", target, err)
	}
}

func TestDecodeRequestJSONRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{`, `{"unknown":true}`, `{"ok":true}{}`, strings.Repeat(" ", 1<<20) + `{}`} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
		var target struct {
			OK bool `json:"ok"`
		}
		if DecodeRequestJSON(httptest.NewRecorder(), request, &target) == nil {
			t.Fatalf("invalid request was accepted: %.40q", body)
		}
	}
}

func TestDecodeFormAcceptsOnlyBoundedSingleValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		body, contentType, query string
		wantError                bool
	}{
		{"quality=original", "application/x-www-form-urlencoded", "", false},
		{"quality=original", "text/plain", "", true},
		{"quality=original", "application/x-www-form-urlencoded", "extra=true", true},
		{"quality=original&quality=720p", "application/x-www-form-urlencoded", "", true},
		{"unknown=true", "application/x-www-form-urlencoded", "", true},
		{strings.Repeat("x", 65), "application/x-www-form-urlencoded", "", true},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/?"+test.query, strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		err := DecodeForm(httptest.NewRecorder(), request, 64, "quality")
		if (err != nil) != test.wantError {
			t.Errorf("DecodeForm(%q, %q, %q) = %v", test.body, test.contentType, test.query, err)
		}
	}
}

func TestFormValueValidation(t *testing.T) { //nolint:cyclop // The table asserts every bounded form-input outcome.
	t.Parallel()
	form := map[string][]string{"required": {"value"}, "optional": {""}}
	if value, ok := RequiredValue(form, "required", 5); !ok || value != "value" {
		t.Fatalf("required value = %q, %t", value, ok)
	}
	if value, ok := OptionalValue(form, "missing", 5); !ok || value != "" {
		t.Fatalf("missing optional value = %q, %t", value, ok)
	}
	if value, ok := OptionalValue(form, "optional", 5); !ok || value != "" {
		t.Fatalf("present optional value = %q, %t", value, ok)
	}
	for _, test := range []struct {
		values   []string
		limit    int
		required bool
		want     bool
	}{
		{[]string{""}, 1, false, true},
		{[]string{""}, 1, true, false},
		{[]string{"long"}, 3, false, false},
		{nil, 3, false, false},
		{[]string{"a", "b"}, 3, false, false},
	} {
		if _, got := BoundedValue(test.values, test.limit, test.required); got != test.want {
			t.Errorf("BoundedValue(%q, %d, %t) = %t", test.values, test.limit, test.required, got)
		}
	}
	if !OnlyFormKeys(form, "required", "optional") || OnlyFormKeys(form, "required") {
		t.Fatal("form key allowlist result was incorrect")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	if !FormEncoded(request) {
		t.Fatal("URL-encoded form was rejected")
	}
	request.Header.Set("Content-Type", "text/plain")
	if FormEncoded(request) {
		t.Fatal("plain text was accepted as a form")
	}
}

func TestEmptyMutationRequestAcceptsOnlyEmptyBodyAndQuery(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, target, body, contentType string
		want                            bool
	}{
		{name: "empty", target: "/", want: true},
		{name: "empty form", target: "/", contentType: "application/x-www-form-urlencoded", want: true},
		{name: "body", target: "/", body: "x"},
		{name: "form", target: "/", body: "x=1", contentType: "application/x-www-form-urlencoded"},
		{name: "query", target: "/?x=1"},
		{name: "bad form", target: "/", body: "%zz", contentType: "application/x-www-form-urlencoded"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.target, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if got := EmptyMutationRequest(httptest.NewRecorder(), request); got != test.want {
				t.Fatalf("EmptyMutationRequest = %t, want %t", got, test.want)
			}
		})
	}
}

func TestDecodeJSONRejectsUnsafeExternalDocuments(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		reader io.Reader
		limit  int64
		strict bool
	}{
		{"oversize", strings.NewReader(strings.Repeat("x", 65)), 64, false},
		{"invalid", strings.NewReader(`{"ok":`), 64, false},
		{"trailing", strings.NewReader(`{"ok":true} {}`), 64, false},
		{"unknown", strings.NewReader(`{"ok":true,"extra":true}`), 64, true},
		{"read error", errorReader{}, 64, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var target struct {
				OK bool `json:"ok"`
			}
			if DecodeJSON(test.reader, test.limit, &target, test.strict) == nil {
				t.Fatal("unsafe document was accepted")
			}
		})
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestLimiterBoundsClients(t *testing.T) {
	t.Parallel()
	var limiter Limiter
	for client := range trackedClientLimit {
		if !limiter.Allow("client-"+strconv.Itoa(client), 1) {
			t.Fatal("limiter rejected a client before its state bound")
		}
	}
	if limiter.Allow("overflow", 1) || len(limiter.windows) != trackedClientLimit {
		t.Fatalf("tracked clients = %d", len(limiter.windows))
	}
}

func TestLimiterPrunesWindowsAndResetsKeys(t *testing.T) {
	t.Parallel()
	var limiter Limiter
	for client := range trackedClientLimit {
		if !limiter.Allow("client-"+strconv.Itoa(client), 1) {
			t.Fatal("limiter rejected a client before its state bound")
		}
	}
	for key, tracked := range limiter.windows {
		tracked.reset = time.Now().Add(-time.Second)
		limiter.windows[key] = tracked
	}
	if !limiter.Allow("replacement", 1) || len(limiter.windows) != 1 || limiter.Allow("replacement", 1) {
		t.Fatalf("pruned clients = %d", len(limiter.windows))
	}
	limiter.Reset("replacement")
	if !limiter.Allow("replacement", 1) || len(limiter.windows) != 1 {
		t.Fatal("reset did not remove one counter")
	}
	if limiter.TrackedClients() != 1 {
		t.Fatalf("tracked clients = %d", limiter.TrackedClients())
	}
}

func TestRemoteHostNormalizesOnlyValidHostPorts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{"192.0.2.1:1234", "192.0.2.1"},
		{"[2001:db8::1]:1234", "2001:db8::1"},
		{"malformed", "malformed"},
		{" malformed ", "malformed"},
	} {
		if got := RemoteHost(test.input); got != test.want {
			t.Errorf("RemoteHost(%q) = %q; want %q", test.input, got, test.want)
		}
	}
}
