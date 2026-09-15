package settings

import (
	"errors"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// ConfigurationField is the storage metadata needed to authorize one change.
type ConfigurationField struct {
	Known, Restart bool
}

// ConfigurationStore adapts app storage to one settings operation.
type ConfigurationStore[S ~string] struct {
	File    string
	Lock    sync.Locker
	Managed func(string) bool
	Source  func(string) S
	Field   func(string) ConfigurationField
	Set     func(string, string, string) error
	Delete  func(string, string) error
	Update  func(string, string, bool)
}

// Change validates and persists one restart configuration field.
func (store ConfigurationStore[S]) Change(key, value string, reset bool) error {
	store.Lock.Lock()
	defer store.Lock.Unlock()
	if store.File == "" {
		return errors.New("configuration storage is unavailable")
	}
	if store.Managed(key) {
		return errors.New(key + " is managed by " + string(store.Source(key)))
	}
	if key == "paths.data" {
		return errors.New("paths.data must be set through YAML or KINOSAIL_DATA_DIR")
	}
	if key == "tls.duckdns" {
		return errors.New("setting is changed through its dedicated settings operation")
	}
	field := store.Field(key)
	if !field.Known || !field.Restart {
		return errors.New("setting is changed through its dedicated settings operation")
	}
	directory := filepath.Dir(store.File)
	if reset {
		if err := store.Delete(directory, key); err != nil {
			return err
		}
		store.Update(key, "", true)
		return nil
	}
	if err := store.Set(directory, key, value); err != nil {
		return err
	}
	store.Update(key, value, false)
	return nil
}

// ParseConfigurationForm decodes one bounded restart configuration form.
func ParseConfigurationForm(writer http.ResponseWriter, request *http.Request, reset bool) (string, string, error) {
	keys := []string{"key"}
	if !reset {
		keys = append(keys, "value")
	}
	if err := httpguard.DecodeForm(writer, request, 17<<10, keys...); err != nil {
		return "", "", errors.New("invalid configuration form")
	}
	keyValues, valueValues := request.PostForm["key"], request.PostForm["value"]
	if len(keyValues) != 1 || keyValues[0] == "" || len(keyValues[0]) > 128 {
		return "", "", errors.New("invalid configuration form")
	}
	if reset {
		return keyValues[0], "", nil
	}
	if len(valueValues) != 1 || valueValues[0] == "" || len(valueValues[0]) > 16<<10 {
		return "", "", errors.New("invalid configuration form")
	}
	return keyValues[0], valueValues[0], nil
}
