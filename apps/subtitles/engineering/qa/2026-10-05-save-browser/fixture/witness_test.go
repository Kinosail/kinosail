package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type publicCue struct {
	Start, End float64
	Text       string
}

type publicReview struct {
	Fingerprint string
	Restorable  bool
	Current     *struct{ Cues []publicCue }
}

type publicReceipt struct {
	ID, Action, Item, State, Outcome string
	Status                           int
}

func savedCues(review publicReview) bool {
	if review.Current == nil {
		return false
	}
	cues := review.Current.Cues
	if len(cues) != 2 {
		return false
	}
	return cues[0] == (publicCue{1, 2, "Fictional reviewed line"}) &&
		cues[1] == (publicCue{4, 5, "Fictional reviewed later line"})
}

func (f *fixture) publicRead(ctx context.Context, original *http.Request, path string, value any) ([]byte, bool) {
 endpoint, err := privateURL(f.authority, path)
 if err != nil { return nil, false }
 request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
 if err != nil { return nil, false }
	request.Header.Set("Origin", f.origin)
	request.Header.Set("Cookie", original.Header.Get("Cookie"))
	request.Header.Set("User-Agent", "R06-private-witness")
	client := f.privateClient(2 * time.Second)
	defer client.CloseIdleConnections()
	if !f.admittedPrivateRequest(request) {
		return nil, false
	}
	response, err := client.Do(request)
	if err != nil {
		f.closeFailedPrivateResponse(response)
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

func (f *fixture) witness(ctx context.Context, request *http.Request, id, operation string, status int) bool {
	if operation == "" && status != http.StatusOK || operation != "" && status != http.StatusAccepted {
		return false
	}
	for {
		if f.observeEffects(ctx, request, id, operation) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (f *fixture) observeEffects(ctx context.Context, request *http.Request, id, operation string) bool {
	effects := f.fileEffects(ctx, request, id)
	once := f.historyOnce(ctx, request, id)
	completed, succeeded := f.receiptEffects(ctx, request, id, operation)
	f.mu.Lock()
	f.state.ActualSaved, f.state.InspectionMatches = effects.saved, effects.inspected
	f.state.RecoveryMatches, f.state.HistoryOnce = effects.recovered, once
	f.state.ReceiptCompleted, f.state.ReceiptSucceeded = completed, succeeded
	f.mu.Unlock()
	// Completed/failed is deliberately not classified as no effects. A nominal
	// successful-response fixture requires success plus the actual effects.
	return effects.saved && effects.inspected && effects.recovered && once && (operation == "" || succeeded)
}

func (f *fixture) recordBrowserRead(request *http.Request, response *capturedResponse) {
	if strings.HasPrefix(request.UserAgent(), "R06-private-") || response.status != http.StatusOK {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if browserCompleted(response, request.URL.Path, f.preparedID, f.preparedItem) {
		f.state.BrowserCompletedReads++
	}
	if browserSaved(response, request.URL.Path) {
		f.state.BrowserSavedInspections++
	}
}

func browserCompleted(response *capturedResponse, path, operation, item string) bool {
	if operation == "" || path != "/api/v1/subtitle-operations/"+operation {
		return false
	}
	var receipt publicReceipt
	if json.Unmarshal(response.body.Bytes(), &receipt) != nil {
		return false
	}
	return completedReceipt(receipt, operation, item) && succeededReceipt(receipt)
}

func browserSaved(response *capturedResponse, path string) bool {
	if !strings.HasSuffix(path, "/inspect") {
		return false
	}
	var view publicReview
	return json.Unmarshal(response.body.Bytes(), &view) == nil && savedCues(view)
}
