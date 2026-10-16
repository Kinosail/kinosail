package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	appbackup "github.com/MikeO7/kinosail-player/internal/backup"
	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestBackupAndRestoreKeepMediaSharesAndAgentConnections(t *testing.T) {
	media, source, restored := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: source, RequireAuth: true})
	var session struct {
		Token string
		TOTP  struct{ Secret string }
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "Backup test", "totp": true}), &session)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, session.TOTP.Secret, time.Now())}), http.StatusOK)
	var library struct{ Items []struct{ ID string } }
	mustJSON(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/library", nil), &library)
	created := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/media-shares", map[string]any{"itemIds": []string{library.Items[0].ID}, "expiresInSeconds": 3600, "maxDevices": 1, "rightsAcknowledged": true})
	var share struct{ ID string }
	mustJSON(t, created, &share)
	if created.Code != http.StatusCreated || share.ID == "" {
		t.Fatalf("media share = %d %q", created.Code, created.Body.String())
	}
	connectAgent(t, handler, session.Token)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/agent-connections", nil), http.StatusOK, `"clientName":"Backup agent"`)
	const key = "correct horse battery staple"
	var archive bytes.Buffer
	if handled, err := appbackup.Command([]string{"backup"}, nil, &archive, source, key); !handled || err != nil {
		t.Fatalf("backup = %t, %v", handled, err)
	}
	if handled, err := appbackup.Command([]string{"restore"}, bytes.NewReader(archive.Bytes()), io.Discard, restored, key); !handled || err != nil {
		t.Fatalf("restore = %t, %v", handled, err)
	}
	restarted := server.New(server.Config{MediaDir: media, DataDir: restored, RequireAuth: true})
	assertAPIBody(t, apiCall(t, restarted, session.Token, http.MethodGet, "/api/v1/media-shares", nil), http.StatusOK, `"id":"`+share.ID+`"`)
	assertAPIBody(t, apiCall(t, restarted, session.Token, http.MethodGet, "/api/v1/agent-connections", nil), http.StatusOK, `"clientName":"Backup agent"`)
}

func connectAgent(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	registered := oauthCall(t, handler, "", http.MethodPost, "/oauth/register", "application/json", `{"client_name":"Backup agent","redirect_uris":["http://127.0.0.1/callback"],"token_endpoint_auth_method":"none","grant_types":["authorization_code","refresh_token"],"response_types":["code"]}`)
	var client struct {
		ClientID string `json:"client_id"`
	}
	mustJSON(t, registered, &client)
	verifier := strings.Repeat("v", 64)
	digest := sha256.Sum256([]byte(verifier))
	query := url.Values{"response_type": {"code"}, "client_id": {client.ClientID}, "redirect_uri": {"http://127.0.0.1/callback"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "code_challenge_method": {"S256"}, "scope": {"kinosail.read"}, "state": {"opaque"}, "resource": {"http://localhost/mcp"}}
	approval := oauthCall(t, handler, token, http.MethodGet, "/oauth/authorize?"+query.Encode(), "", "")
	pending := regexp.MustCompile(`name="request" value="([^"]+)"`).FindStringSubmatch(approval.Body.String())
	if approval.Code != http.StatusOK || pending == nil {
		t.Fatalf("approval = %d %q", approval.Code, approval.Body.String())
	}
	allowed := oauthCall(t, handler, token, http.MethodPost, "/oauth/authorize", "application/x-www-form-urlencoded", url.Values{"request": {pending[1]}, "decision": {"allow"}, "scopes": {"kinosail.read"}}.Encode())
	location, err := allowed.Result().Location()
	if allowed.Code != http.StatusFound || err != nil || location.Query().Get("code") == "" {
		t.Fatalf("authorization = %d %q", allowed.Code, allowed.Body.String())
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {client.ClientID}, "code": {location.Query().Get("code")}, "redirect_uri": {"http://127.0.0.1/callback"}, "code_verifier": {verifier}, "resource": {"http://localhost/mcp"}}
	if exchanged := oauthCall(t, handler, "", http.MethodPost, "/oauth/token", "application/x-www-form-urlencoded", form.Encode()); exchanged.Code != http.StatusOK {
		t.Fatalf("token = %d %q", exchanged.Code, exchanged.Body.String())
	}
}

func oauthCall(t *testing.T, handler http.Handler, token, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
