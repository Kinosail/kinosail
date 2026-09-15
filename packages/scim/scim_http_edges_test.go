package scim

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSCIMProtectRejectsUnconfiguredAndDuplicateSecureAuthorization(t *testing.T) {
	t.Parallel()
	unconfigured := serverWithSCIMConfig(Config{})
	if response := scimCall(t, unconfigured, "", http.MethodGet, "/scim/v2/Users", nil); response.Code != http.StatusNotFound {
		t.Fatalf("unconfigured SCIM = %d %q", response.Code, response.Body.String())
	}
	handler := serverWithSCIM(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users", nil)
	request.TLS = &tls.ConnectionState{}
	request.Header.Add("Authorization", "Bearer "+scimTestToken)
	request.Header.Add("Authorization", "Bearer duplicate")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || response.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("duplicate authorization = %d %q", response.Code, response.Body.String())
	}
}

func TestSCIMCreateAndGetRejectSelectionBeforeRepositorySideEffects(t *testing.T) {
	t.Parallel()
	repository := &edgeRepository{profile: edgeProfile(), found: true}
	api := &scimAPI{repository: repository}
	createRequest := scimEdgeRequest(t, http.MethodPost, "/scim/v2/Users?unknown=1", `{"schemas":["`+scimUserSchema+`"],"userName":"viewer@example.com"}`, "")
	response := httptest.NewRecorder()
	api.createProfile(response, createRequest)
	if response.Code != http.StatusBadRequest || repository.creates != 0 {
		t.Fatalf("invalid create selection = %d creates=%d", response.Code, repository.creates)
	}
	getRequest := scimEdgeRequest(t, http.MethodGet, "/scim/v2/Users/profile?unknown=1", "", "profile")
	response = httptest.NewRecorder()
	api.getProfile(response, getRequest)
	if response.Code != http.StatusBadRequest || repository.gets != 0 {
		t.Fatalf("invalid get selection = %d gets=%d", response.Code, repository.gets)
	}
	createRequest = scimEdgeRequest(t, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+scimUserSchema+`"],"id":"provider-id","userName":"viewer@example.com"}`, "")
	response = httptest.NewRecorder()
	api.createProfile(response, createRequest)
	if response.Code != http.StatusCreated || repository.creates != 1 {
		t.Fatalf("create with ignored ID = %d creates=%d body=%q", response.Code, repository.creates, response.Body.String())
	}
}

func TestSCIMReplaceRejectsEachBoundaryWithoutUpdate(t *testing.T) { //nolint:cyclop // ID, query, body, conditional, and semantic failures all precede persistence.
	t.Parallel()
	tests := []struct {
		name, id, target, body string
		headers                map[string]string
	}{
		{name: "invalid id", id: "bad\n", target: "/scim/v2/Users/profile", body: validSCIMUserBody()},
		{name: "invalid selection", id: "profile", target: "/scim/v2/Users/profile?unknown=1", body: validSCIMUserBody()},
		{name: "invalid body", id: "profile", target: "/scim/v2/Users/profile", body: "{"},
		{name: "invalid conditional", id: "profile", target: "/scim/v2/Users/profile", body: validSCIMUserBody(), headers: map[string]string{"If-Match": "bad"}},
		{name: "invalid profile", id: "profile", target: "/scim/v2/Users/profile", body: `{"schemas":["` + scimUserSchema + `"],"userName":""}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &edgeRepository{profile: edgeProfile(), found: true}
			api := &scimAPI{repository: repository}
			request := scimEdgeRequest(t, http.MethodPut, test.target, test.body, test.id)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			response := httptest.NewRecorder()
			api.replaceProfile(response, request)
			if response.Code != http.StatusBadRequest || repository.updates != 0 {
				t.Fatalf("replace = %d updates=%d body=%q", response.Code, repository.updates, response.Body.String())
			}
		})
	}
	repository := &edgeRepository{profile: edgeProfile(), found: true, updateErr: errors.New("write failed")}
	response := httptest.NewRecorder()
	(&scimAPI{repository: repository}).replaceProfile(response, scimEdgeRequest(t, http.MethodPut, "/scim/v2/Users/profile", validSCIMUserBody(), "profile"))
	if response.Code != http.StatusInternalServerError || repository.updates != 1 {
		t.Fatalf("replace store error = %d updates=%d", response.Code, repository.updates)
	}
}

func TestSCIMPatchRejectsEachBoundaryWithoutUpdate(t *testing.T) { //nolint:cyclop // ID, query, schema, action count, and store failures remain atomic.
	t.Parallel()
	tooMany := make([]map[string]any, maxSCIMPatchActions+1)
	for index := range tooMany {
		tooMany[index] = map[string]any{"op": "replace", "path": "active", "value": true}
	}
	tooManyBody, err := json.Marshal(map[string]any{"schemas": []string{scimPatchSchema}, "Operations": tooMany})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, id, target, body string }{
		{name: "invalid id", id: "bad\n", target: "/scim/v2/Users/profile", body: validSCIMPatchBody()},
		{name: "invalid selection", id: "profile", target: "/scim/v2/Users/profile?unknown=1", body: validSCIMPatchBody()},
		{name: "invalid schema", id: "profile", target: "/scim/v2/Users/profile", body: `{"schemas":["bad"],"Operations":[{"op":"replace","path":"active","value":true}]}`},
		{name: "empty actions", id: "profile", target: "/scim/v2/Users/profile", body: `{"schemas":["` + scimPatchSchema + `"],"Operations":[]}`},
		{name: "too many actions", id: "profile", target: "/scim/v2/Users/profile", body: string(tooManyBody)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &edgeRepository{profile: edgeProfile(), found: true}
			response := httptest.NewRecorder()
			(&scimAPI{repository: repository}).patchProfile(response, scimEdgeRequest(t, http.MethodPatch, test.target, test.body, test.id))
			if response.Code != http.StatusBadRequest || repository.updates != 0 {
				t.Fatalf("patch = %d updates=%d body=%q", response.Code, repository.updates, response.Body.String())
			}
		})
	}
	repository := &edgeRepository{profile: edgeProfile(), found: true, updateErr: errors.New("write failed")}
	response := httptest.NewRecorder()
	(&scimAPI{repository: repository}).patchProfile(response, scimEdgeRequest(t, http.MethodPatch, "/scim/v2/Users/profile", validSCIMPatchBody(), "profile"))
	if response.Code != http.StatusInternalServerError || repository.updates != 1 {
		t.Fatalf("patch store error = %d updates=%d", response.Code, repository.updates)
	}
}

func TestSCIMDeleteRejectsEachBoundaryAndStoreFailure(t *testing.T) { //nolint:cyclop // Invalid delete requests never call persistence.
	t.Parallel()
	for name, request := range map[string]*http.Request{
		"invalid id":    scimEdgeRequest(t, http.MethodDelete, "/scim/v2/Users/profile", "", "bad\n"),
		"invalid query": scimEdgeRequest(t, http.MethodDelete, "/scim/v2/Users/profile?unknown=1", "", "profile"),
	} {
		t.Run(name, func(t *testing.T) {
			repository := &edgeRepository{profile: edgeProfile(), found: true}
			response := httptest.NewRecorder()
			(&scimAPI{repository: repository}).deleteProfile(response, request)
			if response.Code != http.StatusBadRequest || repository.deletes != 0 {
				t.Fatalf("delete = %d deletes=%d", response.Code, repository.deletes)
			}
		})
	}
	repository := &edgeRepository{profile: edgeProfile(), found: true, deleteErr: errors.New("write failed")}
	response := httptest.NewRecorder()
	(&scimAPI{repository: repository}).deleteProfile(response, scimEdgeRequest(t, http.MethodDelete, "/scim/v2/Users/profile", "", "profile"))
	if response.Code != http.StatusInternalServerError || repository.deletes != 1 {
		t.Fatalf("delete store error = %d deletes=%d", response.Code, repository.deletes)
	}
	repository = &edgeRepository{profile: edgeProfile(), found: true}
	request := scimEdgeRequest(t, http.MethodDelete, "/scim/v2/Users/profile", "", "profile")
	request.Header.Set("If-None-Match", `W/"1"`)
	response = httptest.NewRecorder()
	(&scimAPI{repository: repository}).deleteProfile(response, request)
	if response.Code != http.StatusPreconditionFailed || repository.deletes != 0 {
		t.Fatalf("conditional delete = %d deletes=%d", response.Code, repository.deletes)
	}
}

func TestSCIMSchemaDiscoveryRejectsInvalidQuery(t *testing.T) {
	t.Parallel()
	response := scimCall(t, serverWithSCIM(t), scimTestToken, http.MethodGet, "/scim/v2/Schemas/"+scimUserSchema+"?unknown=1", nil)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("schema query = %d %q", response.Code, response.Body.String())
	}
}

func scimEdgeRequest(t *testing.T, method, target, body, id string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/scim+json")
	request.SetPathValue("id", id)
	return request
}

func validSCIMUserBody() string {
	return `{"schemas":["` + scimUserSchema + `"],"userName":"viewer@example.com","displayName":"Viewer"}`
}

func validSCIMPatchBody() string {
	return `{"schemas":["` + scimPatchSchema + `"],"Operations":[{"op":"replace","path":"active","value":false}]}`
}

func edgeProfile() Profile {
	now := time.Now().UTC()
	return Profile{ID: "profile", UserName: "viewer@example.com", Name: "Viewer", CreatedAt: now, UpdatedAt: now, Revision: 1}
}

type edgeRepository struct {
	profile                         Profile
	found                           bool
	updateErr, deleteErr            error
	creates, gets, updates, deletes int
}

func (repository *edgeRepository) List() []Profile { return nil }
func (repository *edgeRepository) Get(string) (Profile, bool) {
	repository.gets++
	return repository.profile, repository.found
}

func (repository *edgeRepository) Create(ProfileInput) (Profile, error) {
	repository.creates++
	return repository.profile, nil
}

func (repository *edgeRepository) Update(string, ProfileInput, string) (Profile, error) {
	repository.updates++
	return repository.profile, repository.updateErr
}

func (repository *edgeRepository) Delete(string, string) error {
	repository.deletes++
	return repository.deleteErr
}
