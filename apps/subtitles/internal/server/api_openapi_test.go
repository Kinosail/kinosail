package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestOpenAPIDocumentsEveryVersionedRouteExactlyOnce(t *testing.T) { //nolint:cyclop,gocognit // One contract test compares every registered route with the document.
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	directory := filepath.Dir(file)
	actual := make(map[string]bool)
	route := regexp.MustCompile(`"(GET|POST|PUT|DELETE|PATCH|HEAD) (/api/v1[^"]*)"`)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, match := range route.FindAllSubmatch(data, -1) {
			actual[string(match[1])+" "+string(match[2])] = true
		}
	}
	spec, err := os.ReadFile(filepath.Join(directory, "api_openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err = json.Unmarshal(spec, &document); err != nil {
		t.Fatal(err)
	}
	documented := make(map[string]bool)
	for path, operations := range document.Paths {
		if count := strings.Count(string(spec), `"`+path+`"`); count != 1 {
			t.Fatalf("OpenAPI path %s occurs %d times", path, count)
		}
		for method := range operations {
			method = strings.ToUpper(method)
			if method != "PARAMETERS" {
				documented[method+" "+path] = true
			}
		}
	}
	if missing, extra := routeDifference(actual, documented), routeDifference(documented, actual); len(missing) != 0 || len(extra) != 0 {
		t.Fatalf("OpenAPI route mismatch\nmissing: %v\nextra: %v", missing, extra)
	}
}

func routeDifference(left, right map[string]bool) []string {
	result := make([]string, 0)
	for route := range left {
		if !right[route] {
			result = append(result, route)
		}
	}
	sort.Strings(result)
	return result
}
