package identitycore

import (
	"crypto/rand"
	"errors"
	"sort"
	"strings"
	"time"
)

// APIKey is the shared persisted API-key metadata.
type APIKey struct {
	Name       string   `json:"name"`
	ProfileID  string   `json:"profileId"`
	Scopes     []string `json:"scopes"`
	CreatedAt  int64    `json:"createdAt"`
	ExpiresAt  int64    `json:"expiresAt"`
	LastUsed   int64    `json:"lastUsed,omitempty"`
	Persistent bool     `json:"persistent,omitempty"`
}

// APIKeyView is the shared API-key presentation model.
type APIKeyView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Scopes   string `json:"scopes"`
	Created  string `json:"created"`
	Expires  string `json:"expires"`
	LastUsed string `json:"lastUsed,omitempty"`
}

// LoadedAPIKeys completes legacy API-key expiry migration after app storage loading.
func LoadedAPIKeys(keys map[string]APIKey, found bool, loadErr error) (map[string]APIKey, error) {
	if loadErr != nil {
		return keys, loadErr
	}
	if !found {
		return keys, nil
	}
	for id, key := range keys {
		if key.ExpiresAt == 0 && !key.Persistent {
			key.ExpiresAt = key.CreatedAt + int64((30*24*time.Hour)/time.Second)
			keys[id] = key
		}
	}
	return keys, nil
}

// IssueAPIKey validates key metadata before generating and committing a secret.
func IssueAPIKey(profileID, name string, scopes []string, persistent bool, commit func(string, APIKey) error) (string, error) {
	if !validAPIKeyInput(profileID, name, scopes, commit) {
		return "", errors.New("API key name and scopes are required")
	}
	for _, scope := range scopes {
		switch scope {
		case "library", "write", "stream", "download", "admin", "home-assistant":
		default:
			return "", errors.New("API key scope must be library, write, stream, download, admin, or home-assistant")
		}
	}
	secret := "ks_" + rand.Text()
	now := time.Now()
	key := APIKey{Name: name, ProfileID: profileID, Scopes: append([]string(nil), scopes...), CreatedAt: now.Unix(), Persistent: persistent}
	if !persistent {
		key.ExpiresAt = now.Add(30 * 24 * time.Hour).Unix()
	}
	if err := commit(secret, key); err != nil {
		return "", err
	}
	return secret, nil
}

func validAPIKeyInput(profileID, name string, scopes []string, commit func(string, APIKey) error) bool {
	return profileID != "" && len(profileID) <= maxIdentityLength && name != "" && len(name) <= 80 && len(scopes) > 0 && len(scopes) <= 5 && commit != nil
}

// ParseAPIScopes validates and canonicalizes a bounded API-key scope list.
func ParseAPIScopes(value string) ([]string, error) {
	if len(value) > 256 {
		return nil, errors.New("API key scope must be library, write, stream, download, or admin")
	}
	allowed := map[string]bool{"library": true, "write": true, "stream": true, "download": true, "admin": true}
	unique := make(map[string]bool)
	for _, scope := range strings.FieldsFunc(value, func(character rune) bool { return character == ',' || character == ' ' }) {
		if !allowed[scope] {
			return nil, errors.New("API key scope must be library, write, stream, download, or admin")
		}
		unique[scope] = true
	}
	result := make([]string, 0, len(unique))
	for scope := range unique {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result, nil
}

// CloneAPIKeys copies API-key metadata and its mutable scopes.
func CloneAPIKeys(keys map[string]APIKey) map[string]APIKey {
	result := make(map[string]APIKey, len(keys))
	for id, key := range keys {
		key.Scopes = append([]string(nil), key.Scopes...)
		result[id] = key
	}
	return result
}

// APIKeyViews formats API-key metadata for settings and API responses.
func APIKeyViews(keys map[string]APIKey) []APIKeyView {
	result := make([]APIKeyView, 0, len(keys))
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		key := keys[id]
		lastUsed := ""
		if key.LastUsed != 0 {
			lastUsed = time.Unix(key.LastUsed, 0).Local().Format("Jan 2, 2006")
		}
		expires := "Never"
		if key.ExpiresAt != 0 {
			expires = time.Unix(key.ExpiresAt, 0).Local().Format("Jan 2, 2006")
		}
		result = append(result, APIKeyView{ID: id, Name: key.Name, Scopes: strings.Join(key.Scopes, ", "), Created: time.Unix(key.CreatedAt, 0).Local().Format("Jan 2, 2006"), Expires: expires, LastUsed: lastUsed})
	}
	sort.SliceStable(result, func(left, right int) bool { return result[left].Created > result[right].Created })
	return result
}
