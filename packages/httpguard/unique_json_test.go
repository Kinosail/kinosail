package httpguard

import (
	"strings"
	"testing"
)

func TestUniqueJSONRejectsAmbiguousAndUnboundedObjects(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"name":"a","name":"b"}`, `{"name":"a","Name":"b"}`, `{"nested":{"x":1,"x":2}}`, `{"unknown":1}`, `{"name":false}`, `{"name":"a"} {}`, `{"name":`, strings.Repeat(" ", 257), "{\"name\":\"\xff\"}"} {
		var result struct {
			Name   string
			Nested struct{ X int }
		}
		if DecodeUniqueJSON(strings.NewReader(raw), 256, &result) == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	var result struct{ Name string }
	if err := DecodeUniqueJSON(strings.NewReader(`{"name":"phone"}`), 256, &result); err != nil || result.Name != "phone" {
		t.Fatalf("valid request = %+v %v", result, err)
	}
}
