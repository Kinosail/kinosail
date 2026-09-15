package mcpgateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type saveErrorStore struct{ err error }

func (store *saveErrorStore) Load(any) (bool, error) { return false, nil }
func (store *saveErrorStore) Save(any) error         { return store.err }

func validRegistrationRequest(t *testing.T) *http.Request {
	t.Helper()
	body := `{"client_name":"Agent","redirect_uris":["http://127.0.0.1/callback"]}`
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/register", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestRegistrationAvailabilityRateAndMediaTypeCauseNoAuthority(t *testing.T) { //nolint:cyclop // The score of 11 remains below the repository ceiling of 22 for the rejection matrix.
	unavailable := NewConnections(ConnectionConfig{})
	response := httptest.NewRecorder()
	unavailable.registerClient(response, validRegistrationRequest(t))
	if response.Code != http.StatusServiceUnavailable || len(unavailable.clients) != 0 {
		t.Fatalf("unavailable registration = %d clients=%d", response.Code, len(unavailable.clients))
	}
	store := &memoryState{}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, store)
	badType := validRegistrationRequest(t)
	badType.Header.Set("Content-Type", "text/plain")
	response = httptest.NewRecorder()
	connections.registerClient(response, badType)
	if response.Code != http.StatusBadRequest || len(connections.clients) != 0 || store.saves != 0 {
		t.Fatalf("media type registration = %d clients=%d saves=%d", response.Code, len(connections.clients), store.saves)
	}
	for index := 1; index < 21; index++ {
		request := validRegistrationRequest(t)
		request.Header.Set("Content-Type", "text/plain")
		connections.registerClient(httptest.NewRecorder(), request)
	}
	response = httptest.NewRecorder()
	connections.registerClient(response, validRegistrationRequest(t))
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" || len(connections.clients) != 0 || store.saves != 0 {
		t.Fatalf("rate limit registration = %d clients=%d saves=%d", response.Code, len(connections.clients), store.saves)
	}
}

func TestRegistrationLimitsAndSaveFailureDoNotCreateClients(t *testing.T) {
	principals := &testPrincipals{values: map[string]Principal{}}
	connections := testConnections("https://kino.test", principals, &memoryState{})
	for index := range mcpConnectionLimit {
		connections.clients[string(rune(index))] = mcpOAuthClient{}
	}
	response := httptest.NewRecorder()
	connections.registerClient(response, validRegistrationRequest(t))
	if response.Code != http.StatusTooManyRequests || len(connections.clients) != mcpConnectionLimit {
		t.Fatalf("client limit registration = %d clients=%d", response.Code, len(connections.clients))
	}
	failed := testConnections("https://kino.test", principals, &memoryState{})
	failed.store = &saveErrorStore{err: errors.New("save failed")}
	response = httptest.NewRecorder()
	failed.registerClient(response, validRegistrationRequest(t))
	if response.Code != http.StatusServiceUnavailable || len(failed.clients) != 0 {
		t.Fatalf("failed registration = %d clients=%d", response.Code, len(failed.clients))
	}
}

func TestRegistrationHelpersRejectDuplicatesAndMalformedRedirects(t *testing.T) {
	if allUnique([]string{"same", "same"}) {
		t.Fatal("duplicate metadata values were accepted")
	}
	for _, redirect := range []string{"", "https://user@example.test/callback", "https://example.test/callback#fragment"} {
		if validMCPRedirect(redirect) {
			t.Fatalf("redirect %q was accepted", redirect)
		}
	}
}

func TestConnectionViewsRevocationAndPruningEdges(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	store := &memoryState{}
	principals := &testPrincipals{values: map[string]Principal{"one": {ID: "one", Name: "One"}, "two": {ID: "two", Name: "Two"}}}
	connections := testConnections("https://kino.test", principals, store)
	connections.now = func() time.Time { return now }
	connections.grants["expired"] = mcpOAuthGrant{ID: "expired", RefreshExpires: now.Add(-time.Second).Unix()}
	connections.grants["two"] = mcpOAuthGrant{ID: "two", ClientName: "Zulu", ProfileID: "two", RefreshExpires: now.Add(time.Hour).Unix()}
	connections.grants["one"] = mcpOAuthGrant{ID: "one", ClientName: "Alpha", ProfileID: "one", RefreshExpires: now.Add(time.Hour).Unix()}
	if views := connections.Views(); len(views) != 2 || views[0].ID != "one" || views[1].ID != "two" {
		t.Fatalf("sorted active views = %#v", views)
	}
	if err := connections.Revoke(""); err == nil {
		t.Fatal("empty connection ID was revoked")
	}
	connections.grants = map[string]mcpOAuthGrant{"grant": {ID: "grant", RefreshExpires: now.Add(time.Hour).Unix()}}
	connections.store = &saveErrorStore{err: errors.New("save failed")}
	if err := connections.Revoke("grant"); err == nil || connections.grants["grant"].ID == "" {
		t.Fatalf("failed revoke = %v grants=%#v", err, connections.grants)
	}
	connections.pending["expired"] = mcpOAuthRequest{Expires: now.Add(-time.Second).Unix()}
	connections.codes["expired"] = mcpOAuthCode{Expires: now.Add(-time.Second).Unix()}
	connections.pruneLocked()
	if len(connections.pending) != 0 || len(connections.codes) != 0 {
		t.Fatalf("pruned pending=%d codes=%d", len(connections.pending), len(connections.codes))
	}
}

func TestPublicMetadataTransportRejectsMalformedAndPrivateDestinations(t *testing.T) {
	client := publicMetadataHTTPClient(time.Millisecond)
	transport := client.Transport.(*http.Transport)
	for _, address := range []string{"invalid", "no-such-host.invalid:443", "127.0.0.1:443", "8.8.8.8:1"} {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		connection, err := transport.DialContext(ctx, "tcp", address)
		cancel()
		if connection != nil {
			_ = connection.Close()
		}
		if err == nil {
			t.Fatalf("metadata dial %q unexpectedly succeeded", address)
		}
	}
	if err := client.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy = %v", err)
	}
}
