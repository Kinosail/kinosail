package federationconfig

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

type samlConfigurationEffects struct {
	managed, source, directory string
	config                     SAMLConfig
	setErr, deleteErr          error
	sets, deletes              int
	updates                    map[string]string
	deleted                    map[string]bool
}

func TestSAMLConfigurationKeysAndView(t *testing.T) { //nolint:cyclop // One test proves the closely related key, view, and configured-state contract.
	t.Parallel()
	keys := SAMLConfigurationKeys()
	keys[0] = "changed"
	if SAMLConfigurationKeys()[0] != SAMLMetadataURLKey {
		t.Fatal("configuration keys exposed shared state")
	}
	values := map[string]struct {
		value      string
		configured bool
	}{
		SAMLMetadataURLKey: {"https://identity.example/metadata", true}, SAMLMetadataXMLKey: {"", false}, SAMLIdentityKey: {"objectGUID", false},
	}
	value := func(key string) (string, bool) { item := values[key]; return item.value, item.configured }
	view := NewSAMLConfigurationView(value, "https://media.example", "control")
	if view.ProviderMetadataURL != values[SAMLMetadataURLKey].value || view.ProviderMetadataXML != "" || view.SPMetadataURL != "https://media.example/login/saml/metadata" || view.ACSURL != "https://media.example/login/saml/acs" || view.IdentityAttribute != "objectGUID" || !view.Configured || view.Control != "control" {
		t.Fatalf("view = %#v", view)
	}
	values[SAMLMetadataURLKey], values[SAMLMetadataXMLKey] = struct {
		value      string
		configured bool
	}{"", false}, struct {
		value      string
		configured bool
	}{"xml", true}
	if !NewSAMLConfigurationView(value, "http://local", 1).Configured {
		t.Fatal("XML configuration was not detected")
	}
	values[SAMLMetadataXMLKey] = struct {
		value      string
		configured bool
	}{"", false}
	if NewSAMLConfigurationView(value, "http://local", 1).Configured {
		t.Fatal("empty configuration was detected")
	}
	stringsByKey := func(key string) string { return values[key].value }
	if SAMLConfigured(stringsByKey) {
		t.Fatal("empty SAML settings were configured")
	}
	values[SAMLMetadataURLKey] = struct {
		value      string
		configured bool
	}{"url", false}
	if !SAMLConfigured(stringsByKey) {
		t.Fatal("SAML URL was not configured")
	}
	values[SAMLMetadataURLKey], values[SAMLMetadataXMLKey] = struct {
		value      string
		configured bool
	}{"", false}, struct {
		value      string
		configured bool
	}{"xml", false}
	if !SAMLConfigured(stringsByKey) {
		t.Fatal("SAML XML was not configured")
	}
}

func TestSAMLConfigurationStoreLifecycle(t *testing.T) { //nolint:cyclop // One lifecycle proves every side-effect boundary.
	t.Parallel()
	effects := &samlConfigurationEffects{}
	store := testSAMLConfigurationStore(effects)
	store.File = ""
	if err := store.Change(SAMLConfig{}, false); err == nil || effects.sets != 0 {
		t.Fatalf("missing storage = %v %#v", err, effects)
	}
	store.File, effects.managed, effects.source = "/data/settings.json", SAMLMetadataXMLKey, "environment"
	if err := store.Change(SAMLConfig{}, false); err == nil || !strings.Contains(err.Error(), "environment") || effects.sets != 0 {
		t.Fatalf("managed = %v %#v", err, effects)
	}
	effects.managed, effects.setErr = "", errors.New("invalid")
	if err := store.Change(SAMLConfig{}, false); !errors.Is(err, effects.setErr) || effects.sets != 1 || len(effects.updates) != 0 {
		t.Fatalf("set failure = %v %#v", err, effects)
	}
	effects.setErr = nil
	config := SAMLConfig{MetadataURL: "https://identity.example/metadata", IdentityAttribute: "NameID"}
	if err := store.Change(config, false); err != nil || effects.directory != "/data" || effects.config != config || effects.updates[SAMLMetadataURLKey] != config.MetadataURL || effects.updates[SAMLMetadataXMLKey] != "" || effects.updates[SAMLIdentityKey] != "NameID" || len(effects.deleted) != 3 {
		t.Fatalf("set = %v %#v", err, effects)
	}
	effects.updates, effects.deleted, effects.deleteErr = map[string]string{}, map[string]bool{}, errors.New("delete")
	if err := store.Change(SAMLConfig{}, true); !errors.Is(err, effects.deleteErr) || effects.deletes != 1 || len(effects.updates) != 0 {
		t.Fatalf("delete failure = %v %#v", err, effects)
	}
	effects.deleteErr = nil
	if err := store.Change(SAMLConfig{}, true); err != nil || effects.deletes != 2 || len(effects.deleted) != 3 {
		t.Fatalf("delete = %v %#v", err, effects)
	}
}

func testSAMLConfigurationStore(effects *samlConfigurationEffects) SAMLConfigurationStore[string] {
	effects.updates, effects.deleted = map[string]string{}, map[string]bool{}
	return SAMLConfigurationStore[string]{
		File: "/data/settings.json", Lock: &sync.Mutex{}, Managed: func(key string) bool { return key == effects.managed }, Source: func(string) string { return effects.source },
		Set: func(directory, metadataURL, metadataXML, identityAttribute string) error {
			effects.sets++
			effects.directory = directory
			effects.config = SAMLConfig{MetadataURL: metadataURL, MetadataXML: metadataXML, IdentityAttribute: identityAttribute}
			return effects.setErr
		},
		Delete: func(directory string) error {
			effects.deletes++
			effects.directory = directory
			return effects.deleteErr
		},
		Update: func(key, value string, deleted bool) { effects.updates[key], effects.deleted[key] = value, deleted },
	}
}

func TestParseSAMLConfigurationForm(t *testing.T) { //nolint:cyclop // The matrix covers each strict field and envelope.
	t.Parallel()
	valid := url.Values{"key": {SAMLConfigurationKey}, "metadataUrl": {"https://identity.example/metadata"}, "metadataXml": {""}, "identityAttribute": {"NameID"}}
	config, err := ParseSAMLConfigurationForm(httptest.NewRecorder(), samlFormRequest(valid))
	if err != nil || config.MetadataURL != valid.Get("metadataUrl") || config.MetadataXML != "" || config.IdentityAttribute != "NameID" {
		t.Fatalf("valid = %#v %v", config, err)
	}
	withXML := cloneOIDCFormValues(valid)
	withXML.Set("metadataXml", "x")
	if config, parseErr := ParseSAMLConfigurationForm(httptest.NewRecorder(), samlFormRequest(withXML)); parseErr != nil || config.MetadataXML != "x" {
		t.Fatalf("XML = %#v %v", config, parseErr)
	}
	tests := map[string]func(url.Values, *http.Request){
		"wrong key": func(v url.Values, _ *http.Request) { v.Set("key", "other") }, "missing URL": func(v url.Values, _ *http.Request) { v.Del("metadataUrl") },
		"large URL": func(v url.Values, _ *http.Request) { v.Set("metadataUrl", strings.Repeat("u", 2049)) }, "missing XML": func(v url.Values, _ *http.Request) { v.Del("metadataXml") },
		"large XML": func(v url.Values, _ *http.Request) { v.Set("metadataXml", strings.Repeat("x", (256<<10)+1)) }, "missing identity": func(v url.Values, _ *http.Request) { v.Del("identityAttribute") },
		"empty identity": func(v url.Values, _ *http.Request) { v.Set("identityAttribute", "") }, "large identity": func(v url.Values, _ *http.Request) { v.Set("identityAttribute", strings.Repeat("i", 257)) },
		"repeat": func(v url.Values, _ *http.Request) { v["metadataUrl"] = []string{"one", "two"} }, "unknown": func(v url.Values, _ *http.Request) { v.Set("unknown", "x") },
		"query": func(_ url.Values, r *http.Request) { r.URL.RawQuery = "x=1" }, "media": func(_ url.Values, r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
	}
	for name, mutate := range tests {
		values := cloneOIDCFormValues(valid)
		request := samlFormRequest(values)
		mutate(values, request)
		if request.URL.RawQuery == "" && request.Header.Get("Content-Type") != "text/plain" {
			request = samlFormRequest(values)
		}
		if _, parseErr := ParseSAMLConfigurationForm(httptest.NewRecorder(), request); parseErr == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestBoundedFormFieldAcceptsExactMaximum(t *testing.T) {
	t.Parallel()
	request := &http.Request{PostForm: url.Values{"field": {"x"}}}
	if value, ok := boundedFormField(request, "field", 1, true); !ok || value != "x" {
		t.Fatalf("boundary = %q %t", value, ok)
	}
}

func TestSaveSAMLConfigurationResultPaths(t *testing.T) { //nolint:cyclop // The result matrix proves each HTTP side-effect path.
	t.Parallel()
	valid := url.Values{"key": {SAMLConfigurationKey}, "metadataUrl": {"https://identity.example/metadata"}, "metadataXml": {""}, "identityAttribute": {"NameID"}}
	for _, test := range []struct {
		name             string
		values           url.Values
		changeErr        error
		wantCode, errors int
	}{{"invalid", url.Values{}, nil, 400, 1}, {"change", valid, errors.New("change"), 409, 1}, {"success", valid, nil, 303, 0}} {
		recorder, failures, calls := httptest.NewRecorder(), 0, 0
		SaveSAMLConfiguration(recorder, samlFormRequest(test.values), func(metadataURL, metadataXML, identity string, reset bool) error {
			calls++
			if metadataURL != valid.Get("metadataUrl") || metadataXML != "" || identity != "NameID" || reset {
				t.Fatal("wrong change input")
			}
			return test.changeErr
		}, func(_ http.ResponseWriter, _ *http.Request, _ string, status int) {
			failures++
			recorder.WriteHeader(status)
		})
		if recorder.Code != test.wantCode || failures != test.errors || calls != boolToInt(test.name != "invalid") || test.name == "success" && recorder.Header().Get("Location") != "/settings/configuration#integrations.saml" {
			t.Fatalf("%s code=%d failures=%d calls=%d", test.name, recorder.Code, failures, calls)
		}
	}
}

func samlFormRequest(values url.Values) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://media.example/settings/configuration", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
