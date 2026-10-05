package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

type witnessedFiles struct{ saved, inspected, recovered bool }

func (f *fixture) fileEffects(ctx context.Context, target *ownedTarget, request *http.Request, id string) witnessedFiles {
	base := "/api/v1/subtitle-library/" + id
	var view publicReview
	_, inspected := f.publicRead(ctx, target, request, base+"/inspect?language=en", &view)
	exported, exportedOK := f.publicRead(ctx, target, request, base+"/export?language=en&format=srt", nil)
	current, currentErr := f.readFixtureSubtitle(false)
	recovery, recoveryErr := f.readFixtureSubtitle(true)
	digest := sha256.Sum256(current)
	return witnessedFiles{
		inspected: inspected && savedCues(view) && view.Restorable && view.Fingerprint == hex.EncodeToString(digest[:]),
		saved:     currentErr == nil && exportedOK && bytes.Equal(current, exported) && !bytes.Equal(current, []byte(initialSRT)) && savedCues(view),
		recovered: recoveryErr == nil && bytes.Equal(recovery, []byte(initialSRT)),
	}
}

func (f *fixture) historyOnce(ctx context.Context, target *ownedTarget, request *http.Request, id string) bool {
	var history struct {
		Matched int
		History []struct{ ID, Action, Reason, Language string }
	}
	_, historyOK := f.publicRead(ctx, target, request, "/api/v1/subtitle-library?view=history", &history)
	if !historyOK || history.Matched != 1 {
		return false
	}
	if len(history.History) != 1 {
		return false
	}
	event := history.History[0]
	return event.ID == id && event.Action == "updated" && event.Reason == "manual" && event.Language == "en"
}

func (f *fixture) receiptEffects(ctx context.Context, target *ownedTarget, request *http.Request, id, operation string) (bool, bool) {
	if operation == "" {
		return false, false
	}
	var receipt publicReceipt
	_, receiptOK := f.publicRead(ctx, target, request, "/api/v1/subtitle-operations/"+operation, &receipt)
	completed := receiptOK && completedReceipt(receipt, operation, id)
	return completed, completed && succeededReceipt(receipt)
}

func completedReceipt(receipt publicReceipt, operation, item string) bool {
	return receipt.ID == operation && receipt.Action == "apply" && receipt.Item == item && receipt.State == "completed"
}

func succeededReceipt(receipt publicReceipt) bool {
	return receipt.Outcome == "success" && receipt.Status == http.StatusOK
}
