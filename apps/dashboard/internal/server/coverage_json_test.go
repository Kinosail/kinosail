package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCoverageJSONCompositeShapes(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  any
		target reflect.Type
		valid  bool
	}{
		{"nil target", "value", nil, false},
		{"map values", map[string]any{"key": "value"}, reflect.TypeFor[map[string]string](), true},
		{"null map value", map[string]any{"key": nil}, reflect.TypeFor[map[string]string](), false},
		{"non map", "value", reflect.TypeFor[map[string]string](), true},
		{"array", []any{"value"}, reflect.TypeFor[[1]string](), true},
		{"null element", []any{nil}, reflect.TypeFor[[]string](), false},
		{"non array", "value", reflect.TypeFor[[]string](), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesJSONShape(test.value, test.target); got != test.valid {
				t.Fatalf("shape = %v", got)
			}
		})
	}
}

func TestCoverageJSONFieldTags(t *testing.T) {
	type fields struct {
		Name    string
		Ignored string `json:"-"`
		private string
	}
	got := jsonFields(reflect.TypeFor[*fields]())
	if len(got) != 1 || got["Name"] != reflect.TypeFor[string]() {
		t.Fatalf("JSON fields = %+v", got)
	}
	value := fields{Name: "Visible", private: "Private value"}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), value.private) {
		t.Fatal("JSON serialization exposed a private field")
	}
	if validJSONShape([]byte("{"), &fields{}) {
		t.Fatal("malformed JSON shape accepted")
	}
}

func TestCoverageJSONTokenFailures(t *testing.T) {
	for _, test := range []struct {
		name, content string
		check         func(*json.Decoder) error
		valid         bool
	}{
		{"object close", "}", checkJSONObject, false},
		{"object missing close", "", checkJSONObject, false},
		{"array invalid value", "{", checkJSONArray, false},
		{"array missing close", "", checkJSONArray, false},
		{"array values", "1,2]", checkJSONArray, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.check(json.NewDecoder(strings.NewReader(test.content)))
			if (err == nil) != test.valid {
				t.Fatalf("token check = %v", err)
			}
		})
	}
	decoder := json.NewDecoder(strings.NewReader("{}"))
	if _, err := decoder.Token(); err != nil {
		t.Fatal(err)
	}
	if err := checkJSONValue(decoder); err == nil {
		t.Fatal("unexpected closing delimiter accepted")
	}
	for _, content := range []string{`{"x":[1,2]}`, `{"x":[`, `{"x":`, `{"x":1`} {
		if got := duplicateJSONKey([]byte(content)); got != (content != `{"x":[1,2]}`) {
			t.Fatalf("invalid token result for %q = %v", content, got)
		}
	}
}

func TestCoverageLoginLimiterCleanup(t *testing.T) {
	limiter := newLoginLimiter()
	now := time.Now()
	limiter.now = func() time.Time { return now }
	for index := 0; index <= maxLoginSources; index++ {
		limiter.sources[string(rune(index))] = loginBucket{until: now.Add(-time.Second)}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.RemoteAddr = "local"
	if allowed, _ := limiter.allow(request, "Owner"); !allowed || len(limiter.sources) != 1 {
		t.Fatalf("cleanup left %d sources", len(limiter.sources))
	}
	if source := loginSource(request, strings.Repeat("A", 90)); source != "local|"+strings.Repeat("a", 80) {
		t.Fatalf("bounded source = %q", source)
	}
}
