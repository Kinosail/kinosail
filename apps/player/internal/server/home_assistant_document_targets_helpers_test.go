package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// These tests exercise the real authenticated application handler and public
// routes. They do not simulate browser storage, claim media playback, or bypass
// CSRF. The canonical API fixture supplies disposable Owner enrollment.
const documentTargetsPath = "/api/v1/home-assistant/players"

var documentTargetID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type documentTargetClient struct {
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
}

type documentTargetCapture struct {
	*httptest.ResponseRecorder
	overflow bool
}

func (capture *documentTargetCapture) Write(body []byte) (int, error) {
	if len(body) > 262144-capture.Body.Len() {
		capture.overflow = true
		return 0, io.ErrShortWrite
	}
	return capture.ResponseRecorder.Write(body)
}

func (capture *documentTargetCapture) WriteString(body string) (int, error) {
	return capture.Write([]byte(body))
}

type documentTargetClaim struct {
	ID        string `json:"id"`
	Claim     string `json:"claim"`
	ExpiresIn int    `json:"expiresIn"`
}

type documentTargetState struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	State    string  `json:"state"`
	Title    string  `json:"title"`
	ItemID   string  `json:"itemId"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Volume   float64 `json:"volume"`
	Muted    bool    `json:"muted"`
}

func newDocumentTargetClient(t *testing.T) *documentTargetClient {
	t.Helper()
	handler, owner := apiServer(t)
	enabled := apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true})
	requireDocumentTargetStatus(t, enabled, http.StatusOK)
	client := &documentTargetClient{handler: handler, cookie: &http.Cookie{
		Name: "__Host-kinosail_session", Value: owner, Secure: true,
		HttpOnly: true, Path: "/", SameSite: http.SameSiteStrictMode,
	}}
	settings := client.call(t, http.MethodGet, "/settings", "", nil)
	requireDocumentTargetStatus(t, settings, http.StatusOK)
	match := regexp.MustCompile(`<meta name="kinosail-csrf" content="([^"]+)">`).FindStringSubmatch(settings.Body.String())
	if len(match) != 2 || len(match[1]) == 0 || len(match[1]) > 256 {
		t.Fatal("R18 authenticated Settings CSRF prerequisite was not met")
	}
	client.csrf = match[1]
	var me struct {
		Viewer struct {
			ID    string `json:"id"`
			Owner bool   `json:"owner"`
		} `json:"viewer"`
	}
	response := client.call(t, http.MethodGet, "/api/v1/me", "", nil)
	requireDocumentTargetStatus(t, response, http.StatusOK)
	readDocumentTargetJSON(t, response, &me)
	if me.Viewer.ID == "" || !me.Viewer.Owner {
		t.Fatal("R18 public current-Owner identity prerequisite was not met")
	}
	return client
}

func (client *documentTargetClient) call(t *testing.T, method, path, body string, claims []string) *httptest.ResponseRecorder {
	t.Helper()
	if len(body) > 4096 {
		t.Fatal("R18 authored request exceeded its body bound")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, method, "https://kinosail.test"+path, strings.NewReader(body))
	defer func() { _ = request.Body.Close() }()
	request.Header.Set("User-Agent", "Kinosail document target regression")
	request.Header.Set("Origin", "https://kinosail.test")
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(client.cookie)
	if client.csrf != "" {
		request.Header.Set("X-Kinosail-CSRF", client.csrf)
	}
	for _, claim := range claims {
		request.Header.Add("X-Kinosail-Player-Claim", claim)
	}
	response := &documentTargetCapture{ResponseRecorder: httptest.NewRecorder()}
	client.handler.ServeHTTP(response, request)
	if ctx.Err() != nil {
		t.Fatal("R18 public request did not settle inside its deadline")
	}
	if response.overflow {
		t.Fatal("R18 public response exceeded its capture bound")
	}
	return response.ResponseRecorder
}

func requireDocumentTargetStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("R18 public response HTTP %d, expected %d", response.Code, want)
	}
}

func readDocumentTargetJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if response.Body.Len() > 16384 {
		t.Fatal("R18 JSON response exceeded its decode bound")
	}
	if json.Unmarshal(response.Body.Bytes(), target) != nil {
		t.Fatal("R18 public response was not the expected JSON")
	}
}

func (client *documentTargetClient) claim(t *testing.T, id string) documentTargetClaim {
	t.Helper()
	body, err := json.Marshal(struct {
		ID string `json:"id,omitempty"`
	}{id})
	if err != nil {
		t.Fatal("R18 authored claim request could not be encoded")
	}
	response := client.call(t, http.MethodPost, documentTargetsPath+"/claims", string(body), nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("R18 claim-route prerequisite: HTTP %d, expected 201", response.Code)
	}
	var claim documentTargetClaim
	readDocumentTargetJSON(t, response, &claim)
	if !documentTargetID.MatchString(claim.ID) || !documentTargetID.MatchString(claim.Claim) {
		t.Fatal("R18 claim response identities violated the public bound")
	}
	if len(claim.Claim) < 20 || claim.ExpiresIn != 30 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("R18 claim response ownership, lease or cache contract was invalid")
	}
	if id != "" && claim.ID != id {
		t.Fatal("R18 claim changed the requested stable candidate")
	}
	return claim
}

func (client *documentTargetClient) snapshot(t *testing.T) map[string]documentTargetState {
	t.Helper()
	response := client.call(t, http.MethodGet, documentTargetsPath, "", nil)
	requireDocumentTargetStatus(t, response, http.StatusOK)
	var result struct {
		Players []documentTargetState `json:"players"`
	}
	readDocumentTargetJSON(t, response, &result)
	if result.Players == nil {
		t.Fatal("R18 target snapshot omitted its public player list")
	}
	players := make(map[string]documentTargetState, len(result.Players))
	for _, player := range result.Players {
		if !documentTargetID.MatchString(player.ID) {
			t.Fatal("R18 published target identity was invalid")
		}
		if _, duplicate := players[player.ID]; duplicate {
			t.Fatal("R18 published snapshot contained duplicate target identities")
		}
		players[player.ID] = player
	}
	return players
}

func requireDocumentTargetSnapshot(t *testing.T, got, want map[string]documentTargetState) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatal("R18 rejected request changed the complete published target snapshot")
	}
}

func (client *documentTargetClient) queueSeek(t *testing.T, id string) {
	t.Helper()
	response := client.call(t, http.MethodPost, documentTargetsPath+"/"+id+"/commands", `{"command":"seek","position":4}`, nil)
	requireDocumentTargetStatus(t, response, http.StatusAccepted)
}

func documentTargetBody(position int) string {
	if position == 9 {
		return `{"name":"Fictional document target","state":"paused","position":9,"duration":12,"volume":0.5}`
	}
	return `{"name":"Fictional document target","state":"paused","position":1,"duration":12,"volume":0.5}`
}

func (client *documentTargetClient) poll(t *testing.T, id string, claims []string, seek bool) {
	t.Helper()
	response := client.call(t, http.MethodPut, documentTargetsPath+"/"+id, documentTargetBody(1), claims)
	requireDocumentTargetStatus(t, response, http.StatusOK)
	var result struct {
		Command  json.RawMessage `json:"command"`
		Position float64         `json:"position"`
	}
	readDocumentTargetJSON(t, response, &result)
	if !seek {
		if string(result.Command) != "null" {
			t.Fatal("R18 unaddressed or already acknowledged target received a command")
		}
		return
	}
	var command string
	if json.Unmarshal(result.Command, &command) != nil || command != "seek" || result.Position != 4 {
		t.Fatal("R18 addressed target did not receive the queued seek exactly")
	}
}

