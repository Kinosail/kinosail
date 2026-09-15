package settings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type configurationEffects struct {
	managed, source        string
	field                  ConfigurationField
	setErr, deleteErr      error
	sets, deletes, updates int
	directory, key, value  string
	deleted                bool
}

func TestConfigurationStoreChangeLifecycle(t *testing.T) { //nolint:cyclop,gocognit // The lifecycle proves every authorization and storage branch.
	t.Parallel()
	tests := []struct {
		name, file, key, value string
		reset                  bool
		mutate                 func(*configurationEffects)
		wantErr                string
		sets, deletes, updates int
	}{
		{"missing storage", "", "backup.retention", "12", false, nil, "configuration storage is unavailable", 0, 0, 0},
		{"managed", "/data/settings.json", "backup.retention", "12", false, func(e *configurationEffects) { e.managed, e.source = "backup.retention", "yaml" }, "backup.retention is managed by yaml", 0, 0, 0},
		{"data path", "/data/settings.json", "paths.data", "/tmp", false, nil, "paths.data must be set through YAML or KINOSAIL_DATA_DIR", 0, 0, 0},
		{"duckdns", "/data/settings.json", "tls.duckdns", "true", false, nil, "setting is changed through its dedicated settings operation", 0, 0, 0},
		{"unknown", "/data/settings.json", "unknown", "value", false, func(e *configurationEffects) { e.field = ConfigurationField{} }, "setting is changed through its dedicated settings operation", 0, 0, 0},
		{"live", "/data/settings.json", "server.name", "value", false, func(e *configurationEffects) { e.field = ConfigurationField{Known: true} }, "setting is changed through its dedicated settings operation", 0, 0, 0},
		{"set failure", "/data/settings.json", "backup.retention", "12", false, func(e *configurationEffects) { e.setErr = errors.New("set failed") }, "set failed", 1, 0, 0},
		{"set", "/data/settings.json", "backup.retention", "12", false, nil, "", 1, 0, 1},
		{"delete failure", "/data/settings.json", "backup.retention", "", true, func(e *configurationEffects) { e.deleteErr = errors.New("delete failed") }, "delete failed", 0, 1, 0},
		{"delete", "/data/settings.json", "backup.retention", "", true, nil, "", 0, 1, 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			effects := &configurationEffects{field: ConfigurationField{Known: true, Restart: true}}
			if test.mutate != nil {
				test.mutate(effects)
			}
			store := testConfigurationStore(effects, test.file)
			err := store.Change(test.key, test.value, test.reset)
			if (test.wantErr == "" && err != nil) || (test.wantErr != "" && (err == nil || err.Error() != test.wantErr)) || effects.sets != test.sets || effects.deletes != test.deletes || effects.updates != test.updates {
				t.Fatalf("err=%v effects=%#v", err, effects)
			}
			if (test.sets+test.deletes) != 0 && (effects.directory != "/data" || effects.key != test.key) {
				t.Fatalf("storage target = %#v", effects)
			}
			if test.updates == 1 && (effects.value != test.value || effects.deleted != test.reset) {
				t.Fatalf("update = %#v", effects)
			}
		})
	}
}

func testConfigurationStore(effects *configurationEffects, file string) ConfigurationStore[string] {
	return ConfigurationStore[string]{
		File: file, Lock: &sync.Mutex{}, Managed: func(key string) bool { return key == effects.managed }, Source: func(string) string { return effects.source },
		Field: func(string) ConfigurationField { return effects.field },
		Set: func(directory, key, value string) error {
			effects.sets++
			effects.directory, effects.key, effects.value = directory, key, value
			return effects.setErr
		},
		Delete: func(directory, key string) error {
			effects.deletes++
			effects.directory, effects.key = directory, key
			return effects.deleteErr
		},
		Update: func(key, value string, deleted bool) {
			effects.updates++
			effects.key, effects.value, effects.deleted = key, value, deleted
		},
	}
}

func TestParseConfigurationForm(t *testing.T) { //nolint:cyclop // The matrix covers the full strict transport boundary.
	t.Parallel()
	valid := url.Values{"key": {"backup.retention"}, "value": {"12"}}
	key, value, err := ParseConfigurationForm(httptest.NewRecorder(), configurationRequest(valid), false)
	if err != nil || key != "backup.retention" || value != "12" {
		t.Fatalf("valid = %q %q %v", key, value, err)
	}
	boundaryKey, boundaryValue := strings.Repeat("k", 128), strings.Repeat("v", 16<<10)
	key, value, err = ParseConfigurationForm(httptest.NewRecorder(), configurationRequest(url.Values{"key": {boundaryKey}, "value": {boundaryValue}}), false)
	if err != nil || key != boundaryKey || value != boundaryValue {
		t.Fatalf("boundary = %d %d %v", len(key), len(value), err)
	}
	resetValues := url.Values{"key": {"backup.retention"}}
	key, value, err = ParseConfigurationForm(httptest.NewRecorder(), configurationRequest(resetValues), true)
	if err != nil || key != "backup.retention" || value != "" {
		t.Fatalf("reset = %q %q %v", key, value, err)
	}
	tests := []struct {
		name   string
		reset  bool
		values url.Values
		alter  func(*http.Request)
	}{
		{"wrong media", false, valid, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }},
		{"query", false, valid, func(r *http.Request) { r.URL.RawQuery = "other=true" }},
		{"missing key", false, url.Values{"value": {"12"}}, nil},
		{"empty key", false, url.Values{"key": {""}, "value": {"12"}}, nil},
		{"large key", false, url.Values{"key": {strings.Repeat("k", 129)}, "value": {"12"}}, nil},
		{"repeat key", false, url.Values{"key": {"one", "two"}, "value": {"12"}}, nil},
		{"missing value", false, resetValues, nil},
		{"empty value", false, url.Values{"key": {"key"}, "value": {""}}, nil},
		{"large value", false, url.Values{"key": {"key"}, "value": {strings.Repeat("v", (16<<10)+1)}}, nil},
		{"repeat value", false, url.Values{"key": {"key"}, "value": {"one", "two"}}, nil},
		{"reset value", true, valid, nil},
		{"unknown", false, url.Values{"key": {"key"}, "value": {"value"}, "other": {"x"}}, nil},
	}
	for _, test := range tests {
		r := configurationRequest(test.values)
		if test.alter != nil {
			test.alter(r)
		}
		if _, _, parseErr := ParseConfigurationForm(httptest.NewRecorder(), r, test.reset); parseErr == nil {
			t.Fatalf("%s accepted", test.name)
		}
	}
}

func configurationRequest(values url.Values) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://media.example/settings/configuration", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
