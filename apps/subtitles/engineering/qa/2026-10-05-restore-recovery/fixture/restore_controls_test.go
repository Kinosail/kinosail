package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRestoreLegacySwapPublicControl(t *testing.T) {
	target := newRestoreTarget(t)
	rig := newRestoreControl(t, target, "none")
	assertRestoreSetup(t, target, rig)
	response := restoreControlPOST(t, target, rig, "/restore", `{"language":"en"}`, "")
	if response.status != http.StatusNoContent || len(response.body) != 0 {
		t.Fatal("legacy Restore must return actual 204 without a body")
	}
	assertRestoredState(t, target, rig)
	assertRestoreCount(t, target, rig, 1)
}

func TestRestorePreparedReceiptPublicControl(t *testing.T) {
	target := newRestoreTarget(t)
	rig := newRestoreControl(t, target, "none")
	assertRestoreSetup(t, target, rig)
	input, err := json.Marshal(map[string]string{"action": "restore", "item": rig.item})
	if err != nil {
		t.Fatal("Restore preparation input unavailable")
	}
	prepared := restoreControlJSON(t, target, rig, http.MethodPost, "/api/v1/subtitle-operations", input, "")
	var receipt restorePublicReceipt
	if prepared.status != http.StatusCreated || json.Unmarshal(prepared.body, &receipt) != nil {
		t.Fatal("Restore preparation must return an actual 201 receipt")
	}
	if !validPreparedRestore(receipt, rig.item) || prepared.header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("Restore preparation identity or privacy unavailable")
	}
	response := restoreControlPOST(t, target, rig, "/restore", `{"language":"en"}`, receipt.ID)
	assertRestoreAccepted(t, response, receipt.ID)
	completed := waitCompletedRestore(t, target, rig, receipt.ID)
	if completed.Action != "restore" || completed.Item != rig.item ||
		completed.Outcome != "success" || completed.Status != http.StatusNoContent {
		t.Fatal("Restore completed receipt must identify actual successful 204")
	}
	assertRestoredState(t, target, rig)
	assertRestoreCount(t, target, rig, 1)
	replay := restoreControlPOST(t, target, rig, "/restore", `{"language":"en"}`, receipt.ID)
	assertRestoreAccepted(t, replay, receipt.ID)
	conflict := restoreControlPOST(t, target, rig, "/restore", `{"language":"fr"}`, receipt.ID)
	if conflict.status != http.StatusConflict {
		t.Fatal("changed-body Restore replay must be rejected")
	}
	assertRestoredState(t, target, rig)
	assertRestoreCount(t, target, rig, 1)
}

func TestRestoreHeldHeadersPublicControl(t *testing.T) {
	target := newRestoreTarget(t)
	rig := newRestoreControl(t, target, "headers")
	assertRestoreSetup(t, target, rig)
	exchange := beginHeldRestore(t, target, rig)
	defer exchange.stop(t)
	state := waitRestoreHold(t, target, rig)
	if !state.HoldEligible || !state.ActualRestored || !state.HistoryOnce ||
		!state.RecoverySwapped || !state.InspectionMatches || state.HeadersReleased {
		t.Fatal("held Restore headers require actual completed effects before release")
	}
	assertRestoredState(t, target, rig)
	response := proveRestoreHeadersPending(t, target, rig, exchange)
	if response.status != http.StatusNoContent || len(response.body) != 0 ||
		!equalRestoreApplicationHeaders(response.header, rig.actualRestoreHeaders()) {
		t.Fatal("released Restore headers or empty actual body changed")
	}
	assertRestoreCount(t, target, rig, 1)
	assertRestoreTransportSettled(t, target, rig)
}

func TestRestoreHeldInspectionBodyPublicControl(t *testing.T) {
	target := newRestoreTarget(t)
	rig := newRestoreControl(t, target, "inspect-body")
	assertRestoreSetup(t, target, rig)
	restored := restoreControlPOST(t, target, rig, "/restore", `{"language":"en"}`, "")
	if restored.status != http.StatusNoContent || len(restored.body) != 0 {
		t.Fatal("inspection body fault requires delivered actual Restore 204")
	}
	assertRestoredState(t, target, rig)
	exchange := beginHeldRestoreInspection(t, target, rig)
	defer exchange.stop(t)
	state := waitRestoreHold(t, target, rig)
	if !state.HoldEligible || !state.ActualRestored || !state.HistoryOnce ||
		!state.RecoverySwapped || !state.InspectionMatches || !state.HeadersReleased {
		t.Fatal("held inspection body requires actual Restore and inspection headers")
	}
	response := proveRestoreInspectionBodyPending(t, target, rig, exchange)
	if response.status != http.StatusOK ||
		!equalRestoreApplicationHeaders(response.header, rig.actualInspectionHeaders()) {
		t.Fatal("released inspection headers changed")
	}
	assertRestoreInspection(t, response.body, rig.item, 1, 4)
	assertRestoreCount(t, target, rig, 1)
	assertRestoreTransportSettled(t, target, rig)
}
