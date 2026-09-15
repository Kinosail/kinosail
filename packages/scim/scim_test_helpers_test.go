package scim

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func serverWithSCIM(t *testing.T) http.Handler {
	t.Helper()
	return serverWithSCIMConfig(Config{Token: scimTestToken, TokenExpiresAt: time.Now().Add(time.Hour)})
}

func serverWithSCIMConfig(config Config) http.Handler {
	mux := http.NewServeMux()
	Register(mux, config, &memoryRepository{})
	return mux
}

func scimCall(t *testing.T, handler http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	return scimCallWithContentType(t, handler, token, method, path, "application/scim+json", body)
}

func scimCallWithContentType(t *testing.T, handler http.Handler, token, method, path, contentType string, body any) *httptest.ResponseRecorder {
	return scimCallWithHeaders(t, handler, token, method, path, body, map[string]string{"Content-Type": contentType})
}

func scimCallWithHeaders(t *testing.T, handler http.Handler, token, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		if raw, ok := body.(json.RawMessage); ok {
			payload = raw
		} else {
			var err error
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(payload))
	request.TLS = &tls.ConnectionState{}
	request.Header.Set("Content-Type", contentType(headers))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func contentType(headers map[string]string) string {
	if value, ok := headers["Content-Type"]; ok {
		return value
	}
	return "application/scim+json"
}

type memoryRepository struct {
	mu       sync.Mutex
	profiles []Profile
	deleted  map[string]Profile
	nextID   uint64
}

func (repository *memoryRepository) List() []Profile {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return append([]Profile(nil), repository.profiles...)
}

func (repository *memoryRepository) Get(id string) (Profile, bool) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, profile := range repository.profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}

func (repository *memoryRepository) Create(input ProfileInput) (Profile, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, profile := range repository.profiles {
		if SameUserName(profile.UserName, input.UserName) || strings.EqualFold(profile.Name, input.Name) {
			return Profile{}, ErrConflict
		}
	}
	if repository.deleted == nil {
		repository.deleted = make(map[string]Profile)
	}
	for id, profile := range repository.deleted {
		if SameUserName(profile.UserName, input.UserName) {
			delete(repository.deleted, id)
			return repository.store(profile, input), nil
		}
	}
	repository.nextID++
	profile := Profile{ID: "scim-" + strconv.FormatUint(repository.nextID, 10), CreatedAt: time.Now().UTC()}
	return repository.store(profile, input), nil
}

func (repository *memoryRepository) Update(id string, input ProfileInput, expected string) (Profile, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for index, profile := range repository.profiles {
		if profile.ID != id {
			continue
		}
		if expected != "" && !ETagMatches(expected, entityTag(profile.Revision)) {
			return Profile{}, ErrPrecondition
		}
		for _, candidate := range repository.profiles {
			if candidate.ID != id && (SameUserName(candidate.UserName, input.UserName) || strings.EqualFold(candidate.Name, input.Name)) {
				return Profile{}, ErrConflict
			}
		}
		profile = profileFromInput(profile, input)
		profile.Revision++
		profile.UpdatedAt = time.Now().UTC()
		repository.profiles[index] = profile
		return profile, nil
	}
	return Profile{}, ErrProfileNotFound
}

func (repository *memoryRepository) Delete(id, expected string) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for index, profile := range repository.profiles {
		if profile.ID != id {
			continue
		}
		if expected != "" && !ETagMatches(expected, entityTag(profile.Revision)) {
			return ErrPrecondition
		}
		if repository.deleted == nil {
			repository.deleted = make(map[string]Profile)
		}
		profile.Revision++
		profile.Disabled = true
		profile.UpdatedAt = time.Now().UTC()
		repository.deleted[id] = profile
		repository.profiles = append(repository.profiles[:index], repository.profiles[index+1:]...)
		return nil
	}
	return ErrProfileNotFound
}

func (repository *memoryRepository) store(profile Profile, input ProfileInput) Profile {
	profile = profileFromInput(profile, input)
	profile.Revision++
	profile.UpdatedAt = time.Now().UTC()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = profile.UpdatedAt
	}
	repository.profiles = append(repository.profiles, profile)
	return profile
}

func profileFromInput(profile Profile, input ProfileInput) Profile {
	profile.Name = input.Name
	profile.UserName = input.UserName
	profile.ExternalID = input.ExternalID
	profile.NameParts = Name{Formatted: input.Formatted, GivenName: input.GivenName, FamilyName: input.FamilyName}
	profile.Emails = append([]Email(nil), input.Emails...)
	profile.Enterprise = input.Enterprise
	profile.Disabled = !input.Active
	return profile
}

func entityTag(revision uint64) string {
	return `W/"` + strconv.FormatUint(revision, 10) + `"`
}
