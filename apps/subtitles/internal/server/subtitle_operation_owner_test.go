package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/httpguard"
)

func TestSubtitleOperationPreservesActiveOwnerAndCSRFBoundaries(t *testing.T) {
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(media, "R06 Example.mp4"), "fictional synthetic media")
	target := filepath.Join(media, "R06 Example.en.srt")
	writeTestFile(t, target, subtitleActionInitial)
	handler := server.New(server.Config{SubtitleApp: true, RequireAuth: true, MediaDir: media, DataDir: data, CacheDir: t.TempDir()})
	owner := signInSubtitleOperationProfile(t, handler, "/setup", "name=Owner&password=fixture-owner-password")
	for _, profile := range []string{"name=Partner&password=fixture-partner-password&owner=true", "name=Viewer&password=fixture-viewer-password"} {
		response := subtitleOperationForm(t, handler, "/settings/profiles", profile, owner)
		if response.Code != http.StatusSeeOther {
			t.Fatalf("disposable profile creation prerequisite = %d", response.Code)
		}
	}
	partner := signInSubtitleOperationProfile(t, handler, "/login", "name=Partner&password=fixture-partner-password")
	viewer := signInSubtitleOperationProfile(t, handler, "/login", "name=Viewer&password=fixture-viewer-password")
	base := subtitleOperationOwnerItem(t, handler, owner)
	item := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	input, _ := json.Marshal(map[string]string{"action": "replacement", "item": item})
	receipt := prepareSubtitleOperationOwnerReceipt(t, handler, string(input), owner)
	statusPath := "/api/v1/subtitle-operations/" + receipt.ID
	assertSubtitleOperationAccessDenied(t, handler, http.MethodGet, statusPath, "", nil, "", http.StatusUnauthorized)
	assertSubtitleOperationAccessDenied(t, handler, http.MethodGet, statusPath, "", viewer, "", http.StatusForbidden)
	assertSubtitleOperationAccessDenied(t, handler, http.MethodGet, statusPath, "", partner, "", http.StatusNotFound)
	assertSubtitleOperationAccessDenied(t, handler, http.MethodGet, statusPath+"/result", "", partner, "", http.StatusNotFound)
	assertSubtitleOperationAccessDenied(t, handler, http.MethodPost, base+"/replacement", `{"replaceable":false}`, partner, receipt.ID, http.StatusNotFound)
	assertSubtitleOperationAccessDenied(t, handler, http.MethodPost, "/api/v1/subtitle-operations", string(input), viewer, "", http.StatusForbidden)
	assertSubtitleOperationCrossOriginDenied(t, handler, string(input), owner)
	assertSubtitleOperationActivationCrossOriginDenied(t, handler, base+"/replacement", receipt.ID, owner)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	receipt = assertSubtitleOperationOwnerPrepared(t, handler, statusPath, owner)
	accepted := subtitleOperationAuthenticated(t, handler, http.MethodPost, base+"/replacement", `{"replaceable":false}`, owner, receipt.ID)
	assertSubtitleOperationAccepted(t, accepted, receipt.ID)
	assertSubtitleOperationOwnerCompletes(t, handler, statusPath, owner)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	assertSubtitleOperationOwnerProtected(t, handler, owner)
}

func prepareSubtitleOperationOwnerReceipt(t *testing.T, handler http.Handler, input string, owner *http.Cookie) subtitleOperationReceipt {
	t.Helper()
	prepared := subtitleOperationAuthenticated(t, handler, http.MethodPost, "/api/v1/subtitle-operations", input, owner, "")
	var receipt subtitleOperationReceipt
	if prepared.Code != http.StatusCreated || json.Unmarshal(prepared.Body.Bytes(), &receipt) != nil || len(receipt.ID) != 64 {
		t.Fatalf("active Owner preparation = %d, want 201 with a receipt", prepared.Code)
	}
	return receipt
}

func assertSubtitleOperationOwnerPrepared(t *testing.T, handler http.Handler, path string, owner *http.Cookie) subtitleOperationReceipt {
	t.Helper()
	current := subtitleOperationAuthenticated(t, handler, http.MethodGet, path, "", owner, "")
	var receipt subtitleOperationReceipt
	if current.Code != http.StatusOK || json.Unmarshal(current.Body.Bytes(), &receipt) != nil || receipt.State != "prepared" {
		t.Fatalf("denied requests changed the active Owner receipt: status %d", current.Code)
	}
	if current.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("Owner receipt status is cacheable")
	}
	return receipt
}

func assertSubtitleOperationOwnerProtected(t *testing.T, handler http.Handler, owner *http.Cookie) {
	t.Helper()
	protected := subtitleOperationAuthenticated(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "", owner, "")
	var inventory struct{ Items []struct{ Frozen bool } }
	if protected.Code != http.StatusOK || json.Unmarshal(protected.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 || !inventory.Items[0].Frozen {
		t.Fatal("active Owner replacement workflow did not protect the existing subtitle")
	}
}

func assertSubtitleOperationActivationCrossOriginDenied(t *testing.T, handler http.Handler, path, operation string, owner *http.Cookie) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(`{"replaceable":false}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Origin", "https://unrelated.invalid")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	request.Header.Set("X-Kinosail-Operation", operation)
	request.AddCookie(owner)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin activation without CSRF = %d", response.Code)
	}
}

func subtitleOperationOwnerItem(t *testing.T, handler http.Handler, owner *http.Cookie) string {
	t.Helper()
	response := subtitleOperationAuthenticated(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "", owner, "")
	var inventory struct{ Items []struct{ ID string } }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 {
		t.Fatalf("active Owner library prerequisite = %d", response.Code)
	}
	return "/api/v1/subtitle-library/" + inventory.Items[0].ID
}

func assertSubtitleOperationAccessDenied(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie, operation string, status int) {
	t.Helper()
	response := subtitleOperationAuthenticated(t, handler, method, path, body, cookie, operation)
	if response.Code != status {
		t.Fatalf("operation authorization rejection = %d, want %d", response.Code, status)
	}
}

func assertSubtitleOperationCrossOriginDenied(t *testing.T, handler http.Handler, body string, owner *http.Cookie) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/subtitle-operations", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Origin", "https://unrelated.invalid")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	request.AddCookie(owner)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin operation without CSRF = %d", response.Code)
	}
}

func assertSubtitleOperationOwnerCompletes(t *testing.T, handler http.Handler, path string, owner *http.Cookie) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response := subtitleOperationAuthenticated(t, handler, http.MethodGet, path, "", owner, "")
		var receipt subtitleOperationReceipt
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &receipt) != nil {
			t.Fatalf("active Owner status = %d", response.Code)
		}
		if receipt.State == "completed" {
			if receipt.Status != http.StatusNoContent || receipt.Outcome != "success" {
				t.Fatalf("active Owner replacement outcome = %+v", receipt)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("active Owner operation did not complete")
}

func subtitleOperationAuthenticated(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie, operation string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
		request.Header.Set("X-Kinosail-CSRF", httpguard.CSRFToken(cookie.Value))
	}
	if operation != "" {
		request.Header.Set("X-Kinosail-Operation", operation)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func signInSubtitleOperationProfile(t *testing.T, handler http.Handler, path, body string) *http.Cookie {
	t.Helper()
	if path == "/setup" {
		body += "&totp=true"
	}
	response := subtitleOperationForm(t, handler, path, body, nil)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("disposable profile sign-in prerequisite = %d", response.Code)
	}
	cookie := cookies[0]
	if path == "/login" && response.Header().Get("Location") != "/account?mfa=required" {
		return cookie
	}
	if path == "/login" {
		response = subtitleOperationForm(t, handler, "/account/mfa/setup", "", cookie)
	}
	secret := regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`).FindStringSubmatch(response.Body.String())
	if len(secret) != 2 {
		t.Fatalf("disposable profile second-factor prerequisite = %d", response.Code)
	}
	confirmed := subtitleOperationForm(t, handler, "/account/mfa/enable", "code="+testTOTP(t, secret[1], time.Now()), cookie)
	if confirmed.Code != http.StatusSeeOther {
		t.Fatalf("disposable profile factor confirmation prerequisite = %d", confirmed.Code)
	}
	return cookie
}

func subtitleOperationForm(t *testing.T, handler http.Handler, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		request.AddCookie(cookie)
		request.Header.Set("X-Kinosail-CSRF", httpguard.CSRFToken(cookie.Value))
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
