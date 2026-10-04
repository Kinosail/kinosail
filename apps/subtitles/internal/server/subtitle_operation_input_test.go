package server_test

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// These public controls protect the receipt preparation boundary. Browser
// deadline cases cannot prove that malformed input starts no server work.
func TestSubtitleOperationPreparationRejectsUntrustedInputWithoutEffects(t *testing.T) {
	t.Parallel()
	handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
	prepared := prepareSubtitleOperation(t, handler, base, "replacement")
	if receipt := readSubtitleOperation(t, handler, prepared.ID); receipt.State != "prepared" {
		t.Fatal("valid preparation did not establish the untrusted-input control")
	}
	item := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	valid := `{"action":"restore","item":"` + item + `"}`
	cases := []struct {
		name, query, body string
		status            int
	}{
		{"unknown field", "", `{"action":"restore","item":"` + item + `","extra":true}`, http.StatusBadRequest},
		{"duplicate action", "", `{"action":"restore","action":"apply","item":"` + item + `"}`, http.StatusBadRequest},
		{"missing action", "", `{"item":"` + item + `"}`, http.StatusBadRequest},
		{"unsupported action", "", `{"action":"delete","item":"` + item + `"}`, http.StatusBadRequest},
		{"missing item", "", `{"action":"restore"}`, http.StatusBadRequest},
		{"uppercase item", "", `{"action":"restore","item":"` + strings.Repeat("A", 16) + `"}`, http.StatusBadRequest},
		{"oversized item", "", `{"action":"restore","item":"` + strings.Repeat("a", 17) + `"}`, http.StatusBadRequest},
		{"unavailable item", "", `{"action":"restore","item":"ffffffffffffffff"}`, http.StatusNotFound},
		{"maintenance item", "", `{"action":"maintain","item":"` + item + `"}`, http.StatusBadRequest},
		{"maintenance explicit null", "", `{"action":"maintain","item":null}`, http.StatusBadRequest},
		{"Wanted explicit null", "", `{"action":"fetch-wanted","item":null}`, http.StatusBadRequest},
		{"array", "", `[]`, http.StatusBadRequest},
		{"empty", "", "", http.StatusBadRequest},
		{"trailing JSON", "", valid + `{}`, http.StatusBadRequest},
		{"oversized body", "", valid + strings.Repeat(" ", 1024), http.StatusBadRequest},
		{"extra query", "?language=en", valid, http.StatusBadRequest},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-operations"+test.query, test.body)
			if response.Code != test.status {
				t.Fatalf("operation preparation rejection = %d, want %d", response.Code, test.status)
			}
			assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
			if _, err := os.Stat(target + ".kinosail.bak"); !os.IsNotExist(err) {
				t.Fatalf("rejected preparation created recovery data: %v", err)
			}
			_ = subtitleActionHistory(t, handler, nil, nil)
		})
	}
}

func TestSubtitleOperationCapacityPreservesExistingPreparedReceipts(t *testing.T) {
	t.Parallel()
	handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
	first := prepareSubtitleOperation(t, handler, base, "replacement")
	for range 63 {
		_ = prepareSubtitleOperation(t, handler, base, "replacement")
	}
	item := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	response := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-operations", `{"action":"replacement","item":"`+item+`"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("full receipt capacity = %d, want 503 before work", response.Code)
	}
	state := readSubtitleOperation(t, handler, first.ID)
	if state.State != "prepared" || state.Status != 0 {
		t.Fatalf("capacity evicted or activated an existing receipt: %+v", state)
	}
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
}
