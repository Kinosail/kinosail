package servertest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type setupCapabilityFixture func(string) http.Handler

func SetupCapability(t *testing.T, newHandler func(string) http.Handler) {
	t.Helper()
	fixture := setupCapabilityFixture(newHandler)
	t.Run("RejectsInvalidInputWithoutStateChange", fixture.rejectsInvalidInputWithoutStateChange)
	t.Run("CreatesOneOwnerUnderConcurrency", fixture.createsOneOwnerUnderConcurrency)
}

func (fixture setupCapabilityFixture) rejectsInvalidInputWithoutStateChange(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"duplicate name", "name=Owner&name=Other&password=owner-password", http.StatusBadRequest},
		{"removed setup code", "name=Owner&password=owner-password&setupCode=unused", http.StatusBadRequest},
		{"legacy setup code", "name=Owner&password=owner-password&setupCode=legacy", http.StatusBadRequest},
		{"unknown field", "name=Owner&password=owner-password&unknown=true", http.StatusBadRequest},
		{"oversized body", "name=Owner&password=" + strings.Repeat("x", (1<<20)+1), http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := fixture(t.TempDir())
			denied := SetupWebRequest(t, handler, "192.0.2.10:1234", test.body)
			if denied.Code != test.status {
				t.Fatalf("invalid web setup = %d %q", denied.Code, denied.Body.String())
			}
			allowed := SetupWebRequest(t, handler, "192.0.2.10:1234", "name=Owner&password=owner-password")
			if allowed.Code != http.StatusSeeOther {
				t.Fatalf("valid setup after denial = %d %q", allowed.Code, allowed.Body.String())
			}
		})
	}

	for _, code := range []string{"legacy", "unused"} {
		t.Run("API "+code, func(t *testing.T) {
			handler := fixture(t.TempDir())
			denied := SetupAPIRequest(t, handler, map[string]any{"name": "Owner", "password": "owner-password", "device": "test", "setupCode": code})
			if denied.Code != http.StatusBadRequest {
				t.Fatalf("removed API setup field %q = %d %q", code, denied.Code, denied.Body.String())
			}
			allowed := SetupAPIRequest(t, handler, map[string]any{"name": "Owner", "password": "owner-password", "device": "test"})
			if allowed.Code != http.StatusCreated {
				t.Fatalf("valid API setup after denial = %d %q", allowed.Code, allowed.Body.String())
			}
		})
	}
}

func (fixture setupCapabilityFixture) createsOneOwnerUnderConcurrency(t *testing.T) {
	t.Parallel()

	handler := fixture(t.TempDir())
	const claims = 8
	responses := make(chan int, claims)
	var group sync.WaitGroup
	for range claims {
		group.Add(1)
		go func() {
			defer group.Done()
			response := SetupAPIRequest(t, handler, map[string]any{"name": "Owner", "password": "owner-password", "device": "test"})
			responses <- response.Code
		}()
	}
	group.Wait()
	close(responses)
	created, conflicts := 0, 0
	for status := range responses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("concurrent setup status = %d", status)
		}
	}
	if created != 1 || conflicts != claims-1 {
		t.Fatalf("created = %d, conflicts = %d", created, conflicts)
	}
}

func SetupAPIRequest(t *testing.T, handler http.Handler, input map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(input); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", &body)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func SetupWebRequest(t *testing.T, handler http.Handler, remote, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = remote
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
