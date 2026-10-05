package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func newRestoreControl(t *testing.T, target *restoreTarget, mode string) *restoreRig {
	t.Helper()
	root, err := os.MkdirTemp(os.Getenv("R06_RESTORE_PRIVATE_ROOT"), "r06-restore-control-")
	if err != nil { t.Fatal("Restore private fixture directory unavailable") }
	f, err := newRestoreRig(target, root, mode)
	if err != nil { t.Fatal("Restore real Server fixture unavailable") }
	t.Cleanup(func() { if !f.stop() { t.Error("Restore owned fixture descriptor did not close") } })
	client, csrf, err := enrollOwner(t, target)
	if err != nil { f.failBoundary(); t.Fatal("Restore actual TLS Owner/MFA/CSRF prerequisite failed") }
	target.installControlOwner(client, csrf)
	catalog := restoreControlGET(t, target, f, "/api/v1/subtitle-library?view=library")
	var data struct { Items []struct { ID string } }
	if catalog.status != http.StatusOK || json.Unmarshal(catalog.body, &data) != nil || len(data.Items) != 1 {
		t.Fatal("Restore actual single-item catalogue unavailable")
	}
	f.item = data.Items[0].ID
	if !restoreItemID.MatchString(f.item) || !target.ownsItem(f.item) { t.Fatal("Restore catalogue ownership unavailable") }
	setupRestoreCurrent(t, target, f)
	return f
}
func setupRestoreCurrent(t *testing.T, target *restoreTarget, f *restoreRig) {
	t.Helper()
	inspected := restoreControlGET(t, target, f, f.inspectPath())
	var before restoreInspection
	if inspected.status != http.StatusOK || json.Unmarshal(inspected.body, &before) != nil || before.ID != f.item ||
		before.Language != "en" || !restoreOperationID.MatchString(before.Fingerprint) {
		t.Fatal("Restore initial public inspection unavailable")
	}
	input, err := json.Marshal(struct {
		Language string `json:"language"`
		Fingerprint string `json:"fingerprint"`
		OffsetMilliseconds int `json:"offsetMilliseconds"`
		AutomaticSync bool `json:"automaticSync"`
		RemoveCredits bool `json:"removeCredits"`
		MergeRepeated bool `json:"mergeRepeated"`
	}{"en", before.Fingerprint, 500, false, false, false})
	if err != nil { t.Fatal("Restore setup Save input unavailable") }
	response := restoreControlJSON(t, target, f, http.MethodPost, "/api/v1/subtitle-library/"+f.item+"/apply", input, "")
	if response.status != http.StatusOK { t.Fatal("Restore actual no-audio setup Save unavailable") }
}

func restoreControlGET(t *testing.T, target *restoreTarget, f *restoreRig, path string) restoreControlResponse {
	t.Helper()
	return restoreControlGETWithin(t, target, f, path, 10*time.Second)
}
func restoreControlGETWithin(t *testing.T, target *restoreTarget, f *restoreRig, path string, remaining time.Duration) restoreControlResponse {
	t.Helper()
	return restoreControlWithin(t, target, f, http.MethodGet, path, nil, "", remaining)
}
func restoreControlPOST(t *testing.T, target *restoreTarget, f *restoreRig, body, operation string) restoreControlResponse {
	t.Helper()
	return restoreControlJSON(t, target, f, http.MethodPost, "/api/v1/subtitle-library/"+f.item+"/restore", []byte(body), operation)
}
func restoreControlJSON(t *testing.T, target *restoreTarget, f *restoreRig, method, path string, body []byte, operation string) restoreControlResponse {
	t.Helper()
	return restoreControlWithin(t, target, f, method, path, body, operation, 10*time.Second)
}
func restoreControlWithin(t *testing.T, target *restoreTarget, f *restoreRig, method, path string, body []byte, operation string, remaining time.Duration) restoreControlResponse {
	t.Helper()
	if remaining <= 0 { return restoreControlResponse{} }
	ctx, cancel := context.WithTimeout(t.Context(), remaining)
	defer cancel()
	endpoint, err := target.endpoint(path)
	if err != nil { return restoreControlResponse{} }
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil { return restoreControlResponse{} }
	request.Header.Set("Origin", target.origin)
	request.Header.Set("User-Agent", "R06-restore-private-control")
	client, csrf := target.controlOwner()
	if client == nil || csrf == "" { return restoreControlResponse{} }
	request.Header.Set("X-Kinosail-CSRF", csrf)
	setRestoreWriteHeaders(request, body, operation)
	if !target.admittedRequest(request) { return restoreControlResponse{} }
	response, err := client.Do(request)
	if err != nil { f.closeFailedResponse(response); return restoreControlResponse{} }
	return f.decodeControlResponse(response)
}
func waitRestoreHold(t *testing.T, target *restoreTarget, f *restoreRig) restoreSnapshot {
	t.Helper()
	end := time.Now().Add(10*time.Second)
	for time.Now().Before(end) {
		response := restoreControlGETWithin(t, target, f, "/__r06_restore/witness", time.Until(end))
		var state restoreSnapshot
		if response.status != http.StatusOK || json.Unmarshal(response.body, &state) != nil { t.Fatal("Restore hold witness unavailable") }
		if state.HoldEligible && state.ActiveHolds == 1 { return state }
		if state.BoundaryFailed || state.HoldExpired { t.Fatal("Restore hold prerequisite failed") }
		time.Sleep(10*time.Millisecond)
	}
	t.Fatal("Restore effects and hold did not become eligible")
	return restoreSnapshot{}
}
func assertRestoreCount(t *testing.T, target *restoreTarget, f *restoreRig) {
	t.Helper()
	response := restoreControlGET(t, target, f, "/api/v1/subtitle-library?view=history")
	var data struct { History []struct { ID, Action, Reason, Language string } }
	if response.status != http.StatusOK || json.Unmarshal(response.body, &data) != nil { t.Fatal("Restore effect count unavailable") }
	count := 0
	for _, event := range data.History {
		if event.ID == f.item && event.Action == "restored" && event.Reason == "restore" && event.Language == "en" { count++ }
	}
	if count != 1 { t.Fatal("Restore replay changed actual History effects") }
}
func assertRestoreTransportSettled(t *testing.T, target *restoreTarget, f *restoreRig) {
	t.Helper()
	end := time.Now().Add(2*time.Second)
	for time.Now().Before(end) {
		response := restoreControlGETWithin(t, target, f, "/__r06_restore/witness", time.Until(end))
		var state restoreSnapshot
		if response.status != http.StatusOK || json.Unmarshal(response.body, &state) != nil { t.Fatal("Restore transport settlement unavailable") }
		delivered := state.RestoreResponseDelivered
		if f.mode == "inspect-body" { delivered = delivered && state.InspectionResponseDelivered }
		if state.ActiveHolds == 0 && delivered && !state.BoundaryFailed && !state.HoldExpired { return }
		time.Sleep(10*time.Millisecond)
	}
	t.Fatal("Restore held transport did not settle")
}

func setRestoreWriteHeaders(request *http.Request, body []byte, operation string) {
	if body != nil { request.Header.Set("Content-Type", "application/json") }
	if operation != "" { request.Header.Set("X-Kinosail-Operation", operation) }
}
func (f *restoreRig) decodeControlResponse(response *http.Response) restoreControlResponse {
	header, status := response.Header.Clone(), response.StatusCode
	data, readErr := readPrivateResponse(response)
	if readErr != nil || len(data) > privateResponseLimit { f.failBoundary(); return restoreControlResponse{} }
	return restoreControlResponse{status: status, header: header, body: data}
}
