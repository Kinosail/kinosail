package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type subtitleOperationReceipt struct {
	ID, Action, Item, State, Outcome string
	Status                           int
}

func TestSubtitlePreparedRestoreActivatesExactlyOnce(t *testing.T) {
	t.Parallel()
	handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
	subtitleOperationSetupSave(t, handler, base)
	shifted, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	receipt := prepareSubtitleOperation(t, handler, base, "restore")
	response := activateSubtitleOperation(t, handler, base+"/restore", `{"language":"en"}`, []string{receipt.ID})
	assertSubtitleOperationAccepted(t, response, receipt.ID)
	completed := waitSubtitleOperation(t, handler, receipt.ID)
	if completed.Action != "restore" || completed.Item != strings.TrimPrefix(base, "/api/v1/subtitle-library/") || completed.Status != http.StatusNoContent || completed.Outcome != "success" {
		t.Fatalf("completed restore receipt = %+v", completed)
	}
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	assertSubtitleActionBytes(t, target+".kinosail.bak", shifted)
	_ = subtitleActionHistory(t, handler, []string{"restored", "updated"}, []string{"restore", "manual"})
	replay := activateSubtitleOperation(t, handler, base+"/restore", `{"language":"en"}`, []string{receipt.ID})
	assertSubtitleOperationAccepted(t, replay, receipt.ID)
	conflict := activateSubtitleOperation(t, handler, base+"/restore", `{"language":"fr"}`, []string{receipt.ID})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("different-body restore replay = %d", conflict.Code)
	}
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	assertSubtitleActionBytes(t, target+".kinosail.bak", shifted)
	_ = subtitleActionHistory(t, handler, []string{"restored", "updated"}, []string{"restore", "manual"})
}

func TestSubtitlePreparedMutationRejectsUnknownOrMalformedReceiptBeforeEffects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		values []string
		status int
	}{
		{"never issued", []string{strings.Repeat("0", 64)}, http.StatusNotFound},
		{"empty supplied header", []string{""}, http.StatusBadRequest},
		{"uppercase", []string{strings.Repeat("A", 64)}, http.StatusBadRequest},
		{"oversized", []string{strings.Repeat("a", 65)}, http.StatusBadRequest},
		{"duplicate header", []string{strings.Repeat("0", 64), strings.Repeat("1", 64)}, http.StatusBadRequest},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
			subtitleOperationSetupSave(t, handler, base)
			current, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			response := activateSubtitleOperation(t, handler, base+"/restore", `{"language":"en"}`, test.values)
			logSubtitleOperationRejectedEffects(t, handler, target, current, []byte(subtitleActionInitial), response.Code)
			if response.Code != test.status {
				t.Fatalf("%s receipt rejection = %d, want %d", test.name, response.Code, test.status)
			}
			assertSubtitleActionBytes(t, target, current)
			assertSubtitleActionBytes(t, target+".kinosail.bak", []byte(subtitleActionInitial))
			_ = subtitleActionHistory(t, handler, []string{"updated"}, []string{"manual"})
		})
	}
}

func logSubtitleOperationRejectedEffects(t *testing.T, handler http.Handler, target string, current, recovery []byte, status int) {
	t.Helper()
	installed, currentErr := os.ReadFile(target)
	backup, recoveryErr := os.ReadFile(target + ".kinosail.bak")
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=history", "")
	var history struct{ Matched int }
	if currentErr != nil || recoveryErr != nil || response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &history) != nil {
		t.Fatal("receipt rejection data-observation prerequisite failed")
	}
	t.Logf("receipt rejection observation: status=%d currentUnchanged=%t recoveryUnchanged=%t historyCount=%d", status, string(installed) == string(current), string(backup) == string(recovery), history.Matched)
}

func TestSubtitlePreparedReceiptCannotActivateAnotherAction(t *testing.T) {
	t.Parallel()
	handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
	subtitleOperationSetupSave(t, handler, base)
	current, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	receipt := prepareSubtitleOperation(t, handler, base, "replacement")
	response := activateSubtitleOperation(t, handler, base+"/restore", `{"language":"en"}`, []string{receipt.ID})
	if response.Code != http.StatusConflict {
		t.Fatalf("wrong action activation = %d", response.Code)
	}
	assertSubtitleActionBytes(t, target, current)
	assertSubtitleActionBytes(t, target+".kinosail.bak", []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, []string{"updated"}, []string{"manual"})
	state := readSubtitleOperation(t, handler, receipt.ID)
	if state.State != "prepared" || state.Action != "replacement" {
		t.Fatalf("mismatched activation consumed receipt: %+v", state)
	}
}

func TestSubtitlePreparedReceiptRejectsStatusQueriesWithoutConsumingActivation(t *testing.T) {
	t.Parallel()
	handler, base, _ := subtitleInspectorFixture(t, subtitleActionInitial)
	receipt := prepareSubtitleOperation(t, handler, base, "replacement")
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+receipt.ID+"?language=en", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("extra status query = %d", response.Code)
	}
	state := readSubtitleOperation(t, handler, receipt.ID)
	if state.State != "prepared" || state.Status != 0 {
		t.Fatalf("status rejection changed the prepared receipt: %+v", state)
	}
}

func subtitleOperationSetupSave(t *testing.T, handler http.Handler, base string) {
	t.Helper()
	before := subtitleActionRead(t, handler, base)
	input, _ := json.Marshal(map[string]any{"language": "en", "fingerprint": before.Fingerprint, "offsetMilliseconds": 500})
	response := requestJSON(t, handler, http.MethodPost, base+"/apply", string(input))
	if response.Code != http.StatusOK {
		t.Fatalf("legacy public Save setup = %d", response.Code)
	}
}

func prepareSubtitleOperation(t *testing.T, handler http.Handler, base, action string) subtitleOperationReceipt {
	t.Helper()
	item := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	input, _ := json.Marshal(map[string]string{"action": action, "item": item})
	response := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-operations", string(input))
	var receipt subtitleOperationReceipt
	if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &receipt) != nil {
		t.Fatalf("public operation preparation = %d, want 201 with a receipt", response.Code)
	}
	if len(receipt.ID) != 64 || strings.Trim(receipt.ID, "0123456789abcdef") != "" || receipt.State != "prepared" || receipt.Action != action || receipt.Item != item {
		t.Fatalf("prepared receipt public shape = %+v", receipt)
	}
	return receipt
}

func activateSubtitleOperation(t *testing.T, handler http.Handler, path, input string, operations []string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(input))
	request.Header.Set("Content-Type", "application/json")
	request.Header["X-Kinosail-Operation"] = append([]string(nil), operations...)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertSubtitleOperationAccepted(t *testing.T, response *httptest.ResponseRecorder, expectedID string) {
	t.Helper()
	var receipt subtitleOperationReceipt
	if response.Code != http.StatusAccepted || json.Unmarshal(response.Body.Bytes(), &receipt) != nil || receipt.ID != expectedID {
		t.Fatalf("public operation activation = %d, want 202 for the prepared receipt", response.Code)
	}
}

func readSubtitleOperation(t *testing.T, handler http.Handler, id string) subtitleOperationReceipt {
	t.Helper()
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+id, "")
	var receipt subtitleOperationReceipt
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &receipt) != nil || receipt.ID != id || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("public operation status = %d, want private receipt", response.Code)
	}
	return receipt
}

func waitSubtitleOperation(t *testing.T, handler http.Handler, id string) subtitleOperationReceipt {
	t.Helper()
	wait := 5 * time.Second
	if readSubtitleOperation(t, handler, id).Action == "audio" {
		wait = subtitleOperationAudioWait
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		receipt := readSubtitleOperation(t, handler, id)
		if receipt.State == "completed" {
			return receipt
		}
		if receipt.State != "running" && receipt.State != "prepared" {
			t.Fatalf("unexpected operation state during execution: %+v", receipt)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("operation did not complete within its bounded public control wait")
	return subtitleOperationReceipt{}
}
