package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

const initialRestoreSRT = "1\n00:00:01,000 --> 00:00:02,000\nFictional original line\n\n2\n00:00:04,000 --> 00:00:05,000\nFictional later line\n"

type restorePublicReceipt struct {
	ID, Action, Item, State, Outcome string
	Status                           int
}

type restoreInspection struct {
	ID, Language, Fingerprint string
	Restorable                bool
	Current                   *struct {
		Cues []struct {
			Start, End float64
			Text       string
		}
	}
}

func assertRestoreSetup(t *testing.T, target *restoreTarget, rig *restoreRig) {
	t.Helper()
	current := rig.readSidecar(t, false)
	backup := rig.readSidecar(t, true)
	if bytes.Equal(current, []byte(initialRestoreSRT)) || !bytes.Equal(backup, []byte(initialRestoreSRT)) {
		t.Fatal("Restore setup must contain changed current and exact original recovery")
	}
	inspected := restoreControlGET(t, target, rig, rig.inspectPath())
	if inspected.status != http.StatusOK {
		t.Fatal("Restore setup public inspection unavailable")
	}
	assertRestoreInspection(t, inspected.body, rig.item, 1.5, 4.5)
	assertRestoreHistory(t, target, rig, false)
	if !bytes.Equal(rig.currentBefore, current) {
		t.Fatal("Restore setup snapshot does not identify actual current bytes")
	}
}

func assertRestoredState(t *testing.T, target *restoreTarget, rig *restoreRig) {
	t.Helper()
	current := rig.readSidecar(t, false)
	backup := rig.readSidecar(t, true)
	if !bytes.Equal(current, []byte(initialRestoreSRT)) || !bytes.Equal(backup, rig.currentBefore) {
		t.Fatal("Restore must swap exact current and recovery bytes")
	}
	exported := restoreControlGET(t, target, rig, rig.exportPath())
	if exported.status != http.StatusOK {
		t.Fatal("Restore public export unavailable")
	}
	inspected := restoreControlGET(t, target, rig, rig.inspectPath())
	if inspected.status != http.StatusOK {
		t.Fatal("Restore public inspection unavailable")
	}
	assertRestoreInspection(t, inspected.body, rig.item, 1, 4)
	var view restoreInspection
	if json.Unmarshal(inspected.body, &view) != nil {
		t.Fatal("Restore public inspection decoding unavailable")
	}
	digest := sha256.Sum256(current)
	if view.Fingerprint != hex.EncodeToString(digest[:]) {
		t.Fatal("Restore public fingerprint does not identify actual current bytes")
	}
	if !validRestoreExport(exported.body, current) {
		t.Fatal("Restore public export does not preserve actual current cues")
	}
	assertRestoreHistory(t, target, rig, true)
}

func assertRestoreInspection(t *testing.T, body []byte, item string, first, later float64) {
	t.Helper()
	var view restoreInspection
	if json.Unmarshal(body, &view) != nil || view.ID != item || view.Language != "en" ||
		!view.Restorable || view.Current == nil || len(view.Current.Cues) != 2 {
		t.Fatal("Restore inspected identity, recovery or cue count invalid")
	}
	assertRestoreInspectionCues(t, view, first, later)
}

func assertRestoreHistory(t *testing.T, target *restoreTarget, rig *restoreRig, restored bool) {
	t.Helper()
	response := restoreControlGET(t, target, rig, "/api/v1/subtitle-library?view=history")
	var history struct {
		Matched int
		History []struct{ ID, Action, Reason, Language string }
	}
	count := 1
	if restored {
		count = 2
	}
	if response.status != http.StatusOK || json.Unmarshal(response.body, &history) != nil ||
		history.Matched != count || len(history.History) != count {
		t.Fatal("Restore History must contain only the actual admitted changes")
	}
	for index, event := range history.History {
		assertRestoreHistoryEvent(t, event.ID, event.Language, event.Action, event.Reason, rig.item, index, restored)
	}
}

func validPreparedRestore(receipt restorePublicReceipt, item string) bool {
	return validRestoreOperationID(receipt.ID) && receipt.Action == "restore" &&
		receipt.Item == item && receipt.State == "prepared" && receipt.Status == 0 && receipt.Outcome == ""
}

func assertRestoreAccepted(t *testing.T, response restoreControlResponse, id string) {
	t.Helper()
	var receipt restorePublicReceipt
	if response.status != http.StatusAccepted || json.Unmarshal(response.body, &receipt) != nil ||
		receipt.ID != id || receipt.Action != "restore" || response.header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("Restore activation must return actual private receipt admission")
	}
}

func waitCompletedRestore(t *testing.T, target *restoreTarget, rig *restoreRig, id string) restorePublicReceipt {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		response := restoreControlGETWithin(t, target, rig, "/api/v1/subtitle-operations/"+id, time.Until(end))
		var receipt restorePublicReceipt
		if response.status != http.StatusOK || json.Unmarshal(response.body, &receipt) != nil ||
			receipt.ID != id || response.header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("Restore public receipt status unavailable")
		}
		if receipt.State == "completed" {
			return receipt
		}
		if receipt.State != "running" && receipt.State != "prepared" {
			t.Fatal("Restore status became uncertain before completed public control")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Restore receipt did not complete within public control bound")
	return restorePublicReceipt{}
}

func validRestoreExport(exported, current []byte) bool {
	return bytes.Equal(bytes.TrimSpace(exported), bytes.TrimSpace(current))
}

func validRestoreOperationID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, letter := range id {
		if letter < '0' || letter > '9' && letter < 'a' || letter > 'f' {
			return false
		}
	}
	return true
}

func assertRestoreInspectionCues(t *testing.T, view restoreInspection, first, later float64) {
	t.Helper()
	for index, cue := range view.Current.Cues {
		at, text := first, "Fictional original line"
		if index == 1 {
			at, text = later, "Fictional later line"
		}
		if cue.Start != at || cue.End != at+1 || cue.Text != text {
			t.Fatal("Restore inspected cues differ from independent expected times and text")
		}
	}
}

func assertRestoreHistoryEvent(t *testing.T, id, language, actualAction, actualReason, item string, index int, restored bool) {
	t.Helper()
	action, reason := "updated", "manual"
	if restored && index == 0 {
		action, reason = "restored", "restore"
	}
	if id != item || language != "en" || actualAction != action || actualReason != reason {
		t.Fatal("Restore History identity or factual action differs")
	}
}
