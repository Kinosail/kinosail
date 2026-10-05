package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type publicCue struct { Start, End float64; Text string }
type publicReview struct {
	Fingerprint string
	Restorable bool
	Current *struct { Cues []publicCue }
}
type publicReceipt struct { ID, Action, Item, State, Outcome string; Status int }

func savedCues(review publicReview) bool {
	return review.Current != nil && len(review.Current.Cues) == 2 &&
		review.Current.Cues[0] == (publicCue{1,2,"Fictional reviewed line"}) &&
		review.Current.Cues[1] == (publicCue{4,5,"Fictional reviewed later line"})
}

func (f *fixture) publicRead(ctx context.Context, original *http.Request, path string, value any) ([]byte, bool) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, f.origin+path, nil)
	if err != nil { return nil, false }
	request.Header.Set("Cookie", original.Header.Get("Cookie"))
	request.Header.Set("Origin", f.origin)
	request.Header.Set("User-Agent", "R06-private-witness")
	client := *f.tls.Client()
	client.Timeout = 2*time.Second
	response, err := client.Do(request)
	if err != nil { return nil, false }
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
	if err != nil || len(data) > privateResponseLimit || response.StatusCode != 200 { return nil, false }
	if value != nil && json.Unmarshal(data, value) != nil { return nil, false }
	return data, true
}

func (f *fixture) witness(ctx context.Context, request *http.Request, id, operation string, status int) bool {
	if operation == "" && status != 200 || operation != "" && status != 202 { return false }
	for {
		eligible := f.observeEffects(ctx, request, id, operation)
		if eligible { return true }
		select {
		case <-ctx.Done(): return false
		case <-time.After(100*time.Millisecond):
		}
	}
}

func (f *fixture) observeEffects(ctx context.Context, request *http.Request, id, operation string) bool {
	base := "/api/v1/subtitle-library/"+id
	var view publicReview
	_, inspected := f.publicRead(ctx, request, base+"/inspect?language=en", &view)
	exported, exportedOK := f.publicRead(ctx, request, base+"/export?language=en&format=srt", nil)
	current, currentErr := os.ReadFile(f.target)
	recovery, recoveryErr := os.ReadFile(f.target+".kinosail.bak")
	digest := sha256.Sum256(current)
	inspection := inspected && savedCues(view) && view.Restorable &&
		view.Fingerprint == hex.EncodeToString(digest[:])
	saved := currentErr == nil && exportedOK && bytes.Equal(current, exported) &&
		!bytes.Equal(current, []byte(initialSRT)) && savedCues(view)
	recovered := recoveryErr == nil && bytes.Equal(recovery, []byte(initialSRT))
	var history struct {
		Matched int
		History []struct { ID, Action, Reason, Language string }
	}
	_, historyOK := f.publicRead(ctx, request, "/api/v1/subtitle-library?view=history", &history)
	once := historyOK && history.Matched == 1 && len(history.History) == 1 &&
		history.History[0].ID == id && history.History[0].Action == "updated" &&
		history.History[0].Reason == "manual" && history.History[0].Language == "en"
	completed, succeeded := false, false
	if operation != "" {
		var receipt publicReceipt
		_, receiptOK := f.publicRead(ctx, request, "/api/v1/subtitle-operations/"+operation, &receipt)
		completed = receiptOK && receipt.ID == operation && receipt.Action == "apply" &&
			receipt.Item == id && receipt.State == "completed"
		succeeded = completed && receipt.Outcome == "success" && receipt.Status == 200
	}
	f.mu.Lock()
	f.state.ActualSaved, f.state.InspectionMatches = saved, inspection
	f.state.RecoveryMatches, f.state.HistoryOnce = recovered, once
	f.state.ReceiptCompleted, f.state.ReceiptSucceeded = completed, succeeded
	f.mu.Unlock()
	// Completed/failed is deliberately not classified as no effects. A nominal
	// successful-response fixture requires success plus the actual effects.
	return saved && inspection && recovered && once && (operation == "" || succeeded)
}

func (f *fixture) recordBrowserRead(request *http.Request, response *capturedResponse) {
	if strings.HasPrefix(request.UserAgent(), "R06-private-") || response.status != 200 { return }
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.preparedID != "" && request.URL.Path == "/api/v1/subtitle-operations/"+f.preparedID {
		var receipt publicReceipt
		if json.Unmarshal(response.body.Bytes(), &receipt) == nil && receipt.ID == f.preparedID &&
			receipt.Action == "apply" && receipt.Item == f.preparedItem &&
			receipt.State == "completed" && receipt.Outcome == "success" && receipt.Status == 200 {
			f.state.BrowserCompletedReads++
		}
	}
	if strings.HasSuffix(request.URL.Path, "/inspect") {
		var view publicReview
		if json.Unmarshal(response.body.Bytes(), &view) == nil && savedCues(view) {
			f.state.BrowserSavedInspections++
		}
	}
}
