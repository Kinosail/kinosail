package scimapp

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	// ConfigurationKey identifies the complete SCIM settings group.
	ConfigurationKey = "integrations.scim"
	// TokenKey stores the SCIM bearer token.
	TokenKey = "integrations.scim.token"
	// ExpirationKey stores the SCIM bearer token expiry.
	ExpirationKey = "integrations.scim.token_expires_at"
)

var configurationKeys = []string{TokenKey, ExpirationKey}

// ConfigurationKeys returns each setting managed as one SCIM configuration.
func ConfigurationKeys() []string { return append([]string(nil), configurationKeys...) }

// ConfigurationState contains the app-owned values used by the shared setup view.
type ConfigurationState struct {
	Origin, Expiration string
	Configured         bool
}

// ConfigurationView contains the shared SCIM setup presentation state.
type ConfigurationView[C any] struct {
	Endpoint, ExpiresOn, MinimumDate, MaximumDate, SuggestedToken string
	Configured, Expired                                           bool
	Control                                                       C
}

// NewConfigurationView derives the complete SCIM setup state.
func NewConfigurationView[C any](state ConfigurationState, control C, request *http.Request) ConfigurationView[C] {
	return configurationView(state, control, request, time.Now().UTC(), rand.Reader)
}

func configurationView[C any](state ConfigurationState, control C, request *http.Request, now time.Time, random io.Reader) ConfigurationView[C] {
	expires, _ := time.Parse(time.RFC3339, state.Expiration)
	expiresOn := expires.UTC().Format(time.DateOnly)
	if state.Expiration == "" {
		expiresOn = now.AddDate(0, 0, 89).Format(time.DateOnly)
	}
	suggested := ""
	if !state.Configured {
		value := make([]byte, 32)
		if _, err := io.ReadFull(random, value); err == nil {
			suggested = base64.RawURLEncoding.EncodeToString(value)
		}
	}
	return ConfigurationView[C]{Endpoint(state.Origin, request), expiresOn, now.Format(time.DateOnly), now.AddDate(0, 0, 90).Format(time.DateOnly), suggested, state.Configured, state.Configured && Expired(state.Expiration, now), control}
}

// Expired reports whether a configured SCIM expiry is invalid or elapsed.
func Expired(raw string, now time.Time) bool {
	expires, err := time.Parse(time.RFC3339, raw)
	return raw != "" && (err != nil || !expires.After(now))
}

// Endpoint returns the configured or request-derived SCIM base URL.
func Endpoint(configured string, request *http.Request) string {
	origin := strings.TrimRight(configured, "/")
	if origin == "" {
		scheme := "http"
		if request.TLS != nil {
			scheme = "https"
		}
		origin = scheme + "://" + request.Host
	}
	return origin + "/scim/v2"
}

// ConfigurationStore adapts app storage to one atomic SCIM settings operation.
type ConfigurationStore[S ~string] struct {
	File    string
	Lock    sync.Locker
	Current func(string) string
	Managed func(string) bool
	Source  func(string) S
	Set     func(string, string, string) error
	Delete  func(string) error
	Update  func(string, string, bool)
}

// Change validates ownership before it persists and projects one SCIM settings change.
func (store ConfigurationStore[S]) Change(token, expiration string, reset bool) error {
	store.Lock.Lock()
	defer store.Lock.Unlock()
	if store.File == "" {
		return errors.New("configuration storage is unavailable")
	}
	for _, key := range configurationKeys {
		if store.Managed(key) {
			return errors.New(key + " is managed by " + string(store.Source(key)))
		}
	}
	directory := filepath.Dir(store.File)
	if reset {
		if err := store.Delete(directory); err != nil {
			return err
		}
		for _, key := range configurationKeys {
			store.Update(key, "", true)
		}
		return nil
	}
	if token == "" {
		token = store.Current(TokenKey)
	}
	if err := store.Set(directory, token, expiration); err != nil {
		return err
	}
	store.Update(TokenKey, token, false)
	store.Update(ExpirationKey, expiration, false)
	return nil
}

// ConfigurationInput is one strictly decoded SCIM settings form.
type ConfigurationInput struct{ Token, Expiration string }

// ParseConfigurationForm decodes one bounded, unambiguous SCIM settings form.
func ParseConfigurationForm(writer http.ResponseWriter, request *http.Request) (ConfigurationInput, error) {
	keys := []string{"key", "token", "tokenExpiresOn"}
	if err := httpguard.DecodeForm(writer, request, 16<<10, keys...); err != nil || request.PostForm.Get("key") != ConfigurationKey {
		return ConfigurationInput{}, errors.New("invalid configuration form")
	}
	token, tokenValues := request.PostForm.Get("token"), request.PostForm["token"]
	expiresOn, expirationValues := request.PostForm.Get("tokenExpiresOn"), request.PostForm["tokenExpiresOn"]
	expires, dateErr := time.Parse(time.DateOnly, expiresOn)
	if len(tokenValues) != 1 || len(token) > 256 || len(expirationValues) != 1 || len(expiresOn) != len(time.DateOnly) || dateErr != nil {
		return ConfigurationInput{}, errors.New("invalid configuration form")
	}
	return ConfigurationInput{token, expires.Add(24*time.Hour - time.Second).UTC().Format(time.RFC3339)}, nil
}

// SaveConfiguration handles one shared SCIM form and delegates its durable change.
func SaveConfiguration(writer http.ResponseWriter, request *http.Request, change func(string, string, bool) error, writeError func(http.ResponseWriter, *http.Request, string, int)) {
	input, err := ParseConfigurationForm(writer, request)
	if err != nil {
		writeError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if err = change(input.Token, input.Expiration, false); err != nil {
		writeError(writer, request, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(writer, request, "/settings/configuration#integrations.scim", http.StatusSeeOther)
}
