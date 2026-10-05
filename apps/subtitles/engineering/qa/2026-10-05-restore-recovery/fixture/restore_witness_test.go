package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func exactRestoreCues(view restoreInspection, first, later float64) bool {
	if view.Language != "en" || view.Current == nil || len(view.Current.Cues) != 2 {
		return false
	}
	cues := view.Current.Cues
	return exactRestoreCue(cues[0].Start, cues[0].End, cues[0].Text, first, "Fictional original line") &&
		exactRestoreCue(cues[1].Start, cues[1].End, cues[1].Text, later, "Fictional later line")
}

func (f *restoreRig) publicRead(ctx context.Context, target *restoreTarget, original *http.Request, path string, value any) ([]byte, bool) {
	endpoint, err := target.endpoint(path)
	if err != nil {
		return nil, false
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false
	}
	request.Header.Set("Origin", target.origin)
	request.Header.Set("Cookie", original.Header.Get("Cookie"))
	request.Header.Set("User-Agent", "R06-restore-private-witness")
	client := target.privateClient(2 * time.Second)
	defer client.CloseIdleConnections()
	if !target.admittedRequest(request) {
		return nil, false
	}
	response, err := client.Do(request)
	if err != nil {
		f.closeFailedResponse(response)
		return nil, false
	}
	data, err := readPrivateResponse(response)
	if err != nil || len(data) > privateResponseLimit || response.StatusCode != http.StatusOK {
		return nil, false
	}
	if value != nil && json.Unmarshal(data, value) != nil {
		return nil, false
	}
	return data, true
}

func (f *restoreRig) fileEffects(ctx context.Context, target *restoreTarget, original *http.Request, id string, restored bool) (bool, bool, []byte) {
	base := "/api/v1/subtitle-library/" + id
	var view restoreInspection
	_, inspected := f.publicRead(ctx, target, original, base+"/inspect?language=en", &view)
	exported, exportedOK := f.publicRead(ctx, target, original, base+"/export?language=en&format=srt", nil)
	current, currentErr := f.readRestoreSubtitle(false)
	backup, backupErr := f.readRestoreSubtitle(true)
	digest := sha256.Sum256(current)
	if !restoreFilesReadable(inspected, exportedOK, currentErr, backupErr) {
		return false, false, nil
	}
	if !restoreFileView(view, id, hex.EncodeToString(digest[:]), exported, current) {
		return false, false, nil
	}
	if !restored {
		valid := setupRestoreFiles(view, current, backup)
		return valid, valid, current
	}
	f.mu.Lock()
	before := append([]byte(nil), f.currentBefore...)
	f.mu.Unlock()
	valid := exactRestoreCues(view, 1, 4) && bytes.Equal(current, []byte(initialRestoreSRT))
	return valid, len(before) > 0 && bytes.Equal(backup, before), current
}

func (f *restoreRig) historyEffects(ctx context.Context, target *restoreTarget, original *http.Request, id string, restored bool) bool {
	var history struct {
		Matched int
		History []struct{ ID, Action, Reason, Language string }
	}
	_, valid := f.publicRead(ctx, target, original, "/api/v1/subtitle-library?view=history", &history)
	count := 1
	if restored {
		count = 2
	}
	if !valid || history.Matched != count || len(history.History) != count {
		return false
	}
	for index, event := range history.History {
		if !expectedRestoreEvent(event.ID, event.Language, event.Action, event.Reason, id, index, restored) {
			return false
		}
	}
	return true
}

func (f *restoreRig) rememberSetup(target *restoreTarget, request *http.Request) bool {
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	stop := f.cancelOnLifecycle(cancel)
	defer stop()
	defer cancel()
	target.mu.Lock()
	id := target.item
	target.mu.Unlock()
	changed, backedUp, current := f.fileEffects(ctx, target, request, id, false)
	if !changed || !backedUp || !f.historyEffects(ctx, target, request, id, false) {
		return false
	}
	f.mu.Lock()
	f.currentBefore = append([]byte(nil), current...)
	f.mu.Unlock()
	return true
}

func completedRestoreReceipt(receipt restorePublicReceipt, operation, item string) bool {
	return receipt.ID == operation && receipt.Action == "restore" && receipt.Item == item && receipt.State == "completed"
}

func successfulRestoreReceipt(receipt restorePublicReceipt) bool {
	return receipt.Outcome == "success" && receipt.Status == http.StatusNoContent
}

func (f *restoreRig) receiptEffects(ctx context.Context, target *restoreTarget, request *http.Request, id, operation string) (bool, bool) {
	if operation == "" {
		return false, false
	}
	var receipt restorePublicReceipt
	_, valid := f.publicRead(ctx, target, request, "/api/v1/subtitle-operations/"+operation, &receipt)
	completed := valid && completedRestoreReceipt(receipt, operation, id)
	return completed, completed && successfulRestoreReceipt(receipt)
}

func (f *restoreRig) witnessRestore(ctx context.Context, target *restoreTarget, request *http.Request, id, operation string, status int) bool {
	if !nominalRestoreResponse(operation, status) {
		return false
	}
	for {
		if f.observeRestoreEffects(ctx, target, request, id, operation) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func nominalRestoreResponse(operation string, status int) bool {
	if operation == "" {
		return status == http.StatusNoContent
	}
	return status == http.StatusAccepted
}

func (f *restoreRig) observeRestoreEffects(ctx context.Context, target *restoreTarget, request *http.Request, id, operation string) bool {
	inspected, recovered, _ := f.fileEffects(ctx, target, request, id, true)
	once := f.historyEffects(ctx, target, request, id, true)
	completed, succeeded := f.receiptEffects(ctx, target, request, id, operation)
	f.mu.Lock()
	f.state.ActualRestored, f.state.InspectionMatches, f.state.RecoverySwapped = inspected, inspected, recovered
	f.state.HistoryOnce, f.state.ReceiptCompleted, f.state.ReceiptSucceeded = once, completed, succeeded
	f.mu.Unlock()
	return inspected && recovered && once && (operation == "" || succeeded)
}

func (f *restoreRig) recordBrowserRead(target *restoreTarget, request *http.Request, response *restoreCapture) {
	if strings.HasPrefix(request.UserAgent(), "R06-restore-private-") || response.status != http.StatusOK || response.overflow {
		return
	}
	target.mu.Lock()
	id, operation := target.item, target.receipt
	target.mu.Unlock()
	if browserRestoreReceipt(response, request.URL.Path, operation, id) {
		f.mu.Lock()
		f.state.BrowserCompletedReceiptReads++
		f.mu.Unlock()
	}
	if browserRestoreInspection(response, request.URL.Path, id) {
		f.mu.Lock()
		f.state.BrowserRestoredInspections++
		f.mu.Unlock()
	}
}

func browserRestoreReceipt(response *restoreCapture, path, operation, id string) bool {
	if operation == "" || path != "/api/v1/subtitle-operations/"+operation {
		return false
	}
	var receipt restorePublicReceipt
	return json.Unmarshal(response.body.Bytes(), &receipt) == nil && completedRestoreReceipt(receipt, operation, id) && successfulRestoreReceipt(receipt)
}

func browserRestoreInspection(response *restoreCapture, path, id string) bool {
	if !strings.HasSuffix(path, "/inspect") {
		return false
	}
	var view restoreInspection
	return json.Unmarshal(response.body.Bytes(), &view) == nil && view.ID == id && exactRestoreCues(view, 1, 4)
}

func exactRestoreCue(start, end float64, text string, expected float64, wanted string) bool {
	return start == expected && end == expected+1 && text == wanted
}

func restoreFilesReadable(inspected, exported bool, currentErr, backupErr error) bool {
	return inspected && exported && currentErr == nil && backupErr == nil
}

func restoreFileView(view restoreInspection, id, digest string, exported, current []byte) bool {
	return view.ID == id && view.Fingerprint == digest && view.Restorable && validRestoreExport(exported, current)
}

func setupRestoreFiles(view restoreInspection, current, backup []byte) bool {
	return exactRestoreCues(view, 1.5, 4.5) && !bytes.Equal(current, []byte(initialRestoreSRT)) && bytes.Equal(backup, []byte(initialRestoreSRT))
}

func expectedRestoreEvent(eventID, language, action, reason, item string, index int, restored bool) bool {
	wantedAction, wantedReason := "updated", "manual"
	if restored && index == 0 {
		wantedAction, wantedReason = "restored", "restore"
	}
	return eventID == item && language == "en" && action == wantedAction && reason == wantedReason
}
