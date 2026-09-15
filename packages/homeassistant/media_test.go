package homeassistant

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMediaHTTP(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "movie.txt")
	if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := &testState{enabled: true, safe: true, items: map[string]Item{"item": {"item", path}}}
	integration := newTestIntegration(t, state)
	expires := state.now.Add(mediaTTL).Unix()
	signature := integration.sign("item", expires)
	target := "/home-assistant/media/item?expires=" + strconv.FormatInt(expires, 10) + "&signature=" + url.QueryEscape(signature)
	req := request(http.MethodGet, target, nil)
	req.SetPathValue("id", "item")
	got := response(http.HandlerFunc(integration.mediaHTTP), req)
	if got.Code != http.StatusOK || got.Body.String() != "media" {
		t.Fatalf("mediaHTTP = %d %q", got.Code, got.Body.String())
	}
	for _, boundary := range []int64{state.now.Unix(), state.now.Add(mediaTTL + time.Minute).Unix()} {
		boundarySignature := integration.sign("item", boundary)
		req = request(http.MethodGet, "/home-assistant/media/item?expires="+strconv.FormatInt(boundary, 10)+"&signature="+url.QueryEscape(boundarySignature), nil)
		req.SetPathValue("id", "item")
		if boundaryResult := response(http.HandlerFunc(integration.mediaHTTP), req); boundaryResult.Code != http.StatusOK {
			t.Errorf("media boundary %d = %d", boundary, boundaryResult.Code)
		}
	}
	for _, suffix := range []string{
		"?expires=bad&signature=" + signature,
		"?expires=" + strconv.FormatInt(expires, 10) + "&signature=bad",
		"?expires=" + strconv.FormatInt(expires, 10) + "&signature=" + signature + "&extra=x",
	} {
		req = request(http.MethodGet, "/home-assistant/media/item"+suffix, nil)
		req.SetPathValue("id", "item")
		got = response(http.HandlerFunc(integration.mediaHTTP), req)
		if got.Code != http.StatusNotFound {
			t.Errorf("mediaHTTP(%q) = %d", suffix, got.Code)
		}
	}
	state.safe = false
	req = request(http.MethodGet, target, nil)
	req.SetPathValue("id", "item")
	got = response(http.HandlerFunc(integration.mediaHTTP), req)
	if got.Code != http.StatusNotFound {
		t.Fatalf("unsafe media = %d", got.Code)
	}
}

func TestValueAndProtocolHelpers(t *testing.T) { //nolint:cyclop // The score of 19 remains below the repository ceiling of 22 for the protocol helper matrix.
	if !allDigits("012345679") || allDigits("01234a67") || !oneOf("a", "a", "b") || oneOf("c", "a", "b") {
		t.Fatal("digit/enum helper mismatch")
	}
	values := url.Values{"a": {"value"}}
	if value, ok := oneValue(values, "a", 4); ok || value != "value" {
		t.Fatalf("bounded value = %q, %v", value, ok)
	}
	if value, ok := oneValue(values, "a", 5); !ok || value != "value" {
		t.Fatalf("boundary value = %q, %v", value, ok)
	}
	if _, ok := oneValue(values, "missing", 5); ok || onlyValues(values, "b") || !onlyValues(values, "a") {
		t.Fatal("form helper mismatch")
	}
	if secretHash("a") == secretHash("b") || validPKCEValue("short") || validPKCEValue(strings.Repeat("?", 43)) || !validPKCEValue(strings.Repeat("A", 43)) || !validPKCEValue(strings.Repeat("A", 128)) {
		t.Fatal("OAuth helper mismatch")
	}
	if formEncoded(request(http.MethodPost, "/", nil)) {
		t.Fatal("missing form content type accepted")
	}
	form := formRequest(http.MethodPost, "/", url.Values{})
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	if !formEncoded(form) {
		t.Fatal("form content type rejected")
	}
}

func TestHTTPErrorHelpers(t *testing.T) {
	recorder := httptest.NewRecorder()
	oauthError(recorder, "invalid_request", http.StatusBadRequest)
	if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Cache-Control") != "no-store" || !strings.Contains(recorder.Body.String(), "invalid_request") {
		t.Fatalf("oauthError = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	apiNotFound(recorder)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("apiNotFound = %d", recorder.Code)
	}
}
