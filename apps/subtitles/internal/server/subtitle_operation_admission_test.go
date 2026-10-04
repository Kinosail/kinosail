package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleExclusiveOperationBlocksAllLegacyWriteAndAudioPathsWithoutEffects(t *testing.T) {
	var providerCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		providerCalls.Add(1)
		http.Error(writer, "fictional provider control must not run", http.StatusServiceUnavailable)
	}))
	t.Cleanup(provider.Close)
	config, target, calls, release := subtitleOperationAudioConfig(t, true)
	config.Subtitles = server.SubtitleConfig{URL: provider.URL, APIKey: "fictional-key"}
	handler := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler)
	subtitleOperationSetupSave(t, handler, base)
	current, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	review := subtitleActionRead(t, handler, base)
	plain, _ := json.Marshal(map[string]any{"language": "en", "fingerprint": review.Fingerprint, "offsetMilliseconds": 500})
	automatic, _ := json.Marshal(map[string]any{"language": "en", "fingerprint": review.Fingerprint, "automaticSync": true})
	prepared := prepareSubtitleOperation(t, handler, base, "audio")
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{prepared.ID}), prepared.ID)
	waitSubtitleOperationProcess(t, calls)
	preview := boundedSubtitleLegacyRequest(t, handler, base+"/preview", string(plain), "application/json")
	if preview.Code != http.StatusOK {
		t.Fatalf("pure read-only preview during running operation = %d", preview.Code)
	}
	id := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	cases := []struct{ name, path, input, contentType string }{
		{"Save", base + "/apply", string(plain), "application/json"},
		{"automatic Save", base + "/apply", string(automatic), "application/json"},
		{"Restore", base + "/restore", `{"language":"en"}`, "application/json"},
		{"Fetch", base + "/fetch", `{"language":"fr"}`, "application/json"},
		{"replacement policy", base + "/replacement", `{"replaceable":true}`, "application/json"},
		{"maintenance", "/api/v1/subtitle-library/maintain", `{"language":"fr","limit":1}`, "application/json"},
		{"Wanted batch", "/api/v1/subtitle-library/fetch-wanted", `{"language":"fr","limit":1}`, "application/json"},
		{"audio", base + "/audio", `{"language":"en"}`, "application/json"},
		{"automatic preview", base + "/preview", string(automatic), "application/json"},
		{"general API Fetch", "/api/v1/items/" + id + "/subtitles", `{"language":"fr"}`, "application/json"},
		{"general web Fetch", "/subtitles/" + id + "/fetch", "", "application/x-www-form-urlencoded"},
		{"web Restore", "/subtitles/manage/" + id + "/restore", "", "application/x-www-form-urlencoded"},
		{"web replacement", "/subtitles/manage/" + id + "/replacement", "replaceable=true", "application/x-www-form-urlencoded"},
		{"web Fetch", "/subtitles/manage/" + id + "/fetch", "", "application/x-www-form-urlencoded"},
		{"web Wanted batch", "/subtitles/manage/fetch-wanted", "", "application/x-www-form-urlencoded"},
		{"web maintenance", "/subtitles/manage/maintain", "", "application/x-www-form-urlencoded"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := boundedSubtitleLegacyRequest(t, handler, test.path, test.input, test.contentType)
			if response.Code != http.StatusConflict {
				t.Fatalf("legacy admission while exclusive work runs = %d, want 409 before work", response.Code)
			}
			assertSubtitleActionBytes(t, target, current)
			assertSubtitleActionBytes(t, target+".kinosail.bak", []byte(subtitleActionInitial))
			assertSubtitleOperationProcessCount(t, calls, 1)
			if providerCalls.Load() != 0 {
				t.Fatalf("rejected legacy work made %d provider calls", providerCalls.Load())
			}
			assertSubtitleOperationFrozen(t, handler, true)
			_ = subtitleActionHistory(t, handler, []string{"updated"}, []string{"manual"})
		})
	}
	writeTestFile(t, release, "release admitted audio")
	_ = waitSubtitleOperation(t, handler, prepared.ID)
}

func TestSubtitleExclusiveOperationDefersReviewedCleanupWithoutRenamingOrChangingSettings(t *testing.T) {
	config, target, calls, release := subtitleOperationAudioConfig(t, true)
	other := strings.TrimSuffix(target, ".en.srt") + ".fr.srt"
	writeTestFile(t, other, "fictional French subtitle retained during admission conflict")
	handler := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler)
	preview := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitles/cleanup/preview", `{"enabled":true,"languages":["en"],"forced":"keep"}`)
	var plan struct {
		Digest string
		Count  int
	}
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &plan) != nil || plan.Count != 1 || len(plan.Digest) != 64 {
		t.Fatal("reviewed cleanup prerequisite did not identify exactly one fictional sidecar")
	}
	beforeSettings := requestApp(t, handler, http.MethodGet, "/api/v1/settings", "").Body.String()
	prepared := prepareSubtitleOperation(t, handler, base, "audio")
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{prepared.ID}), prepared.ID)
	waitSubtitleOperationProcess(t, calls)
	inputs := []struct{ path, body, contentType string }{
		{"/api/v1/subtitles/cleanup", `{"enabled":true,"languages":["en"],"forced":"keep","digest":"` + plan.Digest + `"}`, "application/json"},
		{"/settings/subtitles/cleanup", "enabled=on&language=en&forced=keep&digest=" + plan.Digest, "application/x-www-form-urlencoded"},
	}
	for _, input := range inputs {
		response := boundedSubtitleLegacyRequest(t, handler, input.path, input.body, input.contentType)
		if response.Code != http.StatusConflict {
			t.Fatalf("reviewed cleanup during exclusive work = %d, want 409 before rename/settings work", response.Code)
		}
		assertSubtitleActionBytes(t, other, []byte("fictional French subtitle retained during admission conflict"))
		if _, err := os.Stat(other + ".hidden"); !os.IsNotExist(err) {
			t.Fatal("rejected cleanup created a hidden sidecar")
		}
		if after := requestApp(t, handler, http.MethodGet, "/api/v1/settings", "").Body.String(); after != beforeSettings {
			t.Fatal("rejected cleanup changed public settings")
		}
	}
	assertSubtitleOperationProcessCount(t, calls, 1)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
	writeTestFile(t, release, "release admitted audio")
	_ = waitSubtitleOperation(t, handler, prepared.ID)
}

func TestSubtitleRejectedBodyLeavesPreparedReceiptReusableAndLegacyValidationIntact(t *testing.T) {
	handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
	prepared := prepareSubtitleOperation(t, handler, base, "replacement")
	response := activateSubtitleOperation(t, handler, base+"/replacement", `{"replaceable":null}`, []string{prepared.ID})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid body with a valid receipt = %d", response.Code)
	}
	if state := readSubtitleOperation(t, handler, prepared.ID); state.State != "prepared" || state.Status != 0 {
		t.Fatalf("invalid body consumed or bound the prepared receipt: %+v", state)
	}
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/replacement", `{"replaceable":false}`, []string{prepared.ID}), prepared.ID)
	_ = waitSubtitleOperation(t, handler, prepared.ID)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	assertSubtitleOperationFrozen(t, handler, true)
}

func TestSubtitlePreparedAudioCannotBypassActiveLegacyProviderFetchAdapters(t *testing.T) {
	for _, adapter := range []string{"library", "general"} {
		t.Run(adapter, func(t *testing.T) {
			config, target, calls, _ := subtitleOperationAudioConfig(t, false)
			started, release := make(chan struct{}, 1), make(chan struct{})
			var providerCalls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				providerCalls.Add(1)
				started <- struct{}{}
				select {
				case <-release:
				case <-request.Context().Done():
				}
				http.Error(writer, "fictional held provider control", http.StatusServiceUnavailable)
			}))
			t.Cleanup(provider.Close)
			t.Cleanup(func() { close(release) })
			config.Subtitles = server.SubtitleConfig{URL: provider.URL, APIKey: "fictional-key"}
			handler := server.New(config)
			id := firstSubtitleInventoryID(t, handler)
			base := "/api/v1/subtitle-library/" + id
			prepared := prepareSubtitleOperation(t, handler, base, "audio")
			path := base + "/fetch"
			if adapter == "general" {
				path = "/api/v1/items/" + id + "/subtitles"
			}
			finished := make(chan int, 1)
			go func() {
				request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(`{"language":"fr"}`))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				finished <- response.Code
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("legacy provider prerequisite did not reach the external provider boundary")
			}
			busy := activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{prepared.ID})
			if busy.Code != http.StatusConflict || readSubtitleOperation(t, handler, prepared.ID).State != "prepared" {
				t.Fatalf("exclusive activation bypassed active %s provider adapter: %d", adapter, busy.Code)
			}
			if _, err := os.Stat(calls); !os.IsNotExist(err) || providerCalls.Load() != 1 {
				t.Fatal("busy activation started audio or duplicated the provider request")
			}
			release <- struct{}{}
			select {
			case status := <-finished:
				if status != http.StatusBadGateway {
					t.Fatalf("settled legacy provider error contract = %d", status)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("released legacy provider adapter did not settle")
			}
			assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
			_ = subtitleActionHistory(t, handler, nil, nil)
		})
	}
}

func boundedSubtitleLegacyRequest(t *testing.T, handler http.Handler, path, input, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(input))
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	started := time.Now()
	handler.ServeHTTP(response, request)
	if elapsed := time.Since(started); elapsed >= 250*time.Millisecond {
		t.Fatalf("legacy admission queued instead of returning promptly (%s)", elapsed)
	}
	return response
}

func assertSubtitleOperationFrozen(t *testing.T, handler http.Handler, expected bool) {
	t.Helper()
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	var inventory struct{ Items []struct{ Frozen bool } }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 || inventory.Items[0].Frozen != expected {
		t.Fatalf("public replacement policy = status%d, want one item with frozen=%t", response.Code, expected)
	}
}
