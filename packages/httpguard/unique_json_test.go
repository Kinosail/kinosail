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

func TestUniqueJSONNestedArraysAndDepthLimits(t *testing.T) {
	var target map[string]any
	for _, raw := range []string{`{"values":[1,{"name":"a"},null]}`, `{"values":[]}`} {
		if err := DecodeUniqueJSON(strings.NewReader(raw), 4096, &target); err != nil {
			t.Fatalf("valid nested arrays rejected: %v", err)
		}
	}
	for _, raw := range []string{`{"values":[{"name":"a","NAME":"b"}]}`, `{"values":[{"Setting":1,"ſetting":2}]}`, `{"values":[{"Σ":1,"ς":2}]}`, `{"values":[{"i":1,"İ":2}]}`, `{"values":[`, `{"value":` + strings.Repeat(`[`, 34) + `0` + strings.Repeat(`]`, 34) + `}`} {
		if DecodeUniqueJSON(strings.NewReader(raw), 4096, &target) == nil {
			t.Fatalf("invalid nested object accepted: %q", raw)
		}
	}
	if DecodeUniqueJSON(nil, 1, &target) == nil || DecodeUniqueJSON(strings.NewReader(`{}`), 0, &target) == nil {
		t.Fatal("invalid JSON reader bounds accepted")
	}
}

func TestUniqueJSONPreservesUnambiguousUnicodeMapKeys(t *testing.T) {
	var target map[string]map[string]int
	raw := `{"values":{"ſetting":1,"ß":2,"ss":3,"é":4,"e\u0301":5}}`
	if err := DecodeUniqueJSON(strings.NewReader(raw), 256, &target); err != nil {
		t.Fatal(err)
	}
	values := target["values"]
	if len(values) != 5 || values["ſetting"] != 1 || values["ß"] != 2 || values["ss"] != 3 || values["é"] != 4 || values["e\u0301"] != 5 {
		t.Fatalf("unambiguous keys changed: %#v", values)
	}
}
