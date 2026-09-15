package servertest

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // TOTP test helper follows RFC 6238.
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// CurrentTOTP returns the current RFC 6238 test code for a valid shared secret.
func CurrentTOTP(secret string) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	var counter [8]byte
	//nolint:gosec // Current Unix time is nonnegative.
	binary.BigEndian.PutUint64(counter[:], uint64(time.Now().Unix()/30))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}

// RouteJSON sends a JSON request while preserving the all-routes fixture semantics.
func RouteJSON(t *testing.T, handler http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if err := json.NewEncoder(&payload).Encode(body); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), method, path, &payload)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// CreateRouteAPIKey creates and validates the scoped key used by route inventory tests.
func CreateRouteAPIKey(t *testing.T, handler http.Handler, owner, scope string) string {
	t.Helper()
	response := RouteJSON(t, handler, owner, http.MethodPost, "/api/v1/api-keys", map[string]string{"name": scope + " route test", "scopes": scope})
	var result struct {
		Secret string `json:"secret"`
	}
	if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Secret == "" {
		t.Fatalf("create %s API key = %d %q", scope, response.Code, response.Body.String())
	}
	return result.Secret
}
