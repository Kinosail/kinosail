package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleAudioResultsStayBoundedAndUnavailableResultsDoNotRestartWork(t *testing.T) {
	config, target, calls, _ := subtitleOperationAudioConfig(t, false)
	var seconds atomic.Int64
	start := time.Now()
	config.SubtitleOperationTime.Now = func() time.Time { return start.Add(time.Duration(seconds.Load()) * time.Second) }
	handler := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler)
	ids := make([]string, 0, 3)
	for range 3 {
		prepared := prepareSubtitleOperation(t, handler, base, "audio")
		assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{prepared.ID}), prepared.ID)
		_ = waitSubtitleOperation(t, handler, prepared.ID)
		ids = append(ids, prepared.ID)
	}
	first := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+ids[0]+"/result", "")
	if first.Code != http.StatusNotFound {
		t.Fatalf("third result did not enforce the two-result bound: %d", first.Code)
	}
	if receipt := readSubtitleOperation(t, handler, ids[0]); receipt.State != "completed" || receipt.Outcome != "success" {
		t.Fatal("result eviction erased the factual completed outcome")
	}
	for _, id := range ids[1:] {
		result := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+id+"/result", "")
		if result.Code != http.StatusOK || result.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("retained private audio result = %d", result.Code)
		}
		assertSubtitleOperationAudioResult(t, result.Body.Bytes())
	}
	query := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+ids[2]+"/result?format=json", "")
	if query.Code != http.StatusBadRequest {
		t.Fatalf("audio result with unsupported query = %d", query.Code)
	}
	seconds.Store(31 * 60)
	expired := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+ids[2]+"/result", "")
	if expired.Code != http.StatusNotFound {
		t.Fatalf("expired audio result = %d", expired.Code)
	}
	assertSubtitleOperationProcessCount(t, calls, 1)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
}

func TestSubtitleCompletedAudioResultRetainsOwnerBinding(t *testing.T) {
	config, target, calls, _ := subtitleOperationAudioConfig(t, false)
	config.RequireAuth = true
	handler := server.New(config)
	owner := signInSubtitleOperationProfile(t, handler, "/setup", "name=Owner&password=fixture-owner-password")
	created := subtitleOperationForm(t, handler, "/settings/profiles", "name=Partner&password=fixture-partner-password&owner=true", owner)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("disposable second Owner prerequisite = %d", created.Code)
	}
	partner := signInSubtitleOperationProfile(t, handler, "/login", "name=Partner&password=fixture-partner-password")
	base := subtitleOperationOwnerItem(t, handler, owner)
	item := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	prepared := subtitleOperationAuthenticated(t, handler, http.MethodPost, "/api/v1/subtitle-operations", `{"action":"audio","item":"`+item+`"}`, owner, "")
	var receipt subtitleOperationReceipt
	if prepared.Code != http.StatusCreated || json.Unmarshal(prepared.Body.Bytes(), &receipt) != nil {
		t.Fatalf("Owner audio preparation = %d", prepared.Code)
	}
	accepted := subtitleOperationAuthenticated(t, handler, http.MethodPost, base+"/audio", `{"language":"en"}`, owner, receipt.ID)
	assertSubtitleOperationAccepted(t, accepted, receipt.ID)
	waitSubtitleOperationOwnerAudio(t, handler, receipt.ID, owner)
	resultPath := "/api/v1/subtitle-operations/" + receipt.ID + "/result"
	assertSubtitleOperationAccessDenied(t, handler, http.MethodGet, resultPath, "", nil, "", http.StatusUnauthorized)
	assertSubtitleOperationAccessDenied(t, handler, http.MethodGet, resultPath, "", partner, "", http.StatusNotFound)
	result := subtitleOperationAuthenticated(t, handler, http.MethodGet, resultPath, "", owner, "")
	if result.Code != http.StatusOK || result.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("bound Owner audio result = %d", result.Code)
	}
	assertSubtitleOperationAudioResult(t, result.Body.Bytes())
	assertSubtitleOperationProcessCount(t, calls, 1)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
}

func waitSubtitleOperationOwnerAudio(t *testing.T, handler http.Handler, id string, owner *http.Cookie) {
	t.Helper()
	deadline := time.Now().Add(subtitleAudioFixtureSettlementLimit)
	for time.Now().Before(deadline) {
		response := subtitleOperationAuthenticated(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+id, "", owner, "")
		var receipt subtitleOperationReceipt
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &receipt) != nil {
			t.Fatalf("Owner audio status = %d", response.Code)
		}
		if receipt.State == "completed" {
			if receipt.Status != http.StatusOK || receipt.Outcome != "success" {
				t.Fatalf("Owner audio outcome = %+v", receipt)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Owner audio control did not settle within the declared fixture bound")
}
