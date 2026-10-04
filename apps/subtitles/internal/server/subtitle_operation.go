package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// SubtitleOperationTime supplies external time only to prepared receipts.
// Production defaults retain real time and lifecycle-derived deadlines.
type SubtitleOperationTime struct {
	Now         func() time.Time
	WithTimeout func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

type subtitleOperations struct {
	mu          sync.Mutex
	admission   *subtitleAdmission
	clock       SubtitleOperationTime
	lifecycle   context.Context
	path        string
	available   bool
	records     map[string]subtitleOperationRecord
	active      map[string]context.Context
	results     map[string][]byte
	resultOrder []string
}

func newSubtitleOperations(config Config) *subtitleOperations {
	clock := config.SubtitleOperationTime
	if clock.Now == nil {
		clock.Now = time.Now
	}
	if clock.WithTimeout == nil {
		clock.WithTimeout = context.WithTimeout
	}
	lifecycle := config.Lifecycle
	if lifecycle == nil {
		lifecycle = context.Background()
	}
	operations := &subtitleOperations{clock: clock, lifecycle: lifecycle, admission: &subtitleAdmission{}, records: map[string]subtitleOperationRecord{}, active: map[string]context.Context{}, results: map[string][]byte{}}
	operations.load(config.DataDir)
	if !operations.available {
		slog.Error("subtitle operation registry unavailable", "outcome", "receipt-service-unavailable")
	}
	return operations
}

func validSubtitleOperationInput(action, item string) bool {
	if oneOf(action, "maintain", "fetch-wanted") {
		return item == ""
	}
	return oneOf(action, "apply", "restore", "fetch", "replacement", "audio") && validSubtitleItemID(item)
}

func (operations *subtitleOperations) prepare(request *http.Request, action, item string) (subtitleOperationReceipt, int) {
	operations.mu.Lock()
	defer operations.mu.Unlock()
	if !operations.available || operations.lifecycle.Err() != nil {
		return subtitleOperationReceipt{}, http.StatusServiceUnavailable
	}
	operations.expire()
	if len(operations.records) >= 64 {
		return subtitleOperationReceipt{}, http.StatusServiceUnavailable
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return subtitleOperationReceipt{}, http.StatusServiceUnavailable
	}
	now := operations.clock.Now().Unix()
	receipt := subtitleOperationReceipt{ID: hex.EncodeToString(bytes), Action: action, Item: item, State: "prepared"}
	operations.records[receipt.ID] = subtitleOperationRecord{subtitleOperationReceipt: receipt, Owner: currentViewer(request).ID, Created: now, Expires: now + int64((5*time.Minute)/time.Second)}
	if operations.persist() != nil {
		delete(operations.records, receipt.ID)
		operations.persistenceFailed(request, action, receipt.ID, "preparation-not-issued")
		return subtitleOperationReceipt{}, http.StatusServiceUnavailable
	}
	return receipt, http.StatusCreated
}

func (operations *subtitleOperations) record(request *http.Request, id string) (subtitleOperationRecord, int) {
	if !operations.available {
		return subtitleOperationRecord{}, http.StatusServiceUnavailable
	}
	operations.expire()
	record, found := operations.records[id]
	if !validSubtitleFingerprint(id) || !found || record.Owner != currentViewer(request).ID {
		return subtitleOperationRecord{}, http.StatusNotFound
	}
	return record, http.StatusOK
}

func (operations *subtitleOperations) read(request *http.Request, id string) (subtitleOperationReceipt, int) {
	operations.mu.Lock()
	defer operations.mu.Unlock()
	record, status := operations.record(request, id)
	if ctx, ok := operations.active[id]; ok && ctx.Err() != nil {
		record.Outcome = "cancellation-requested"
	}
	return record.subtitleOperationReceipt, status
}

func subtitleOperationDeadline(action string) time.Duration {
	switch action {
	case "restore", "replacement":
		return time.Minute
	case "audio":
		return 21 * time.Minute
	case "apply", "fetch":
		return 30 * time.Minute
	default:
		return 2 * time.Hour
	}
}

func (operations *subtitleOperations) rejected(writer http.ResponseWriter, request *http.Request, action string, status int) {
	slog.Warn("subtitle operation rejected", "request_id", activityRequestID(request), "action", subtitleOperationLogAction(action), "status", status, "outcome", "not-started")
	writeSubtitleOperationJSON(writer, map[string]string{"error": "subtitle operation is unavailable or cannot be started; review its status and current subtitle"}, status)
}

func (operations *subtitleOperations) persistenceFailed(request *http.Request, action, id, outcome string) {
	slog.Error("subtitle operation persistence failed", "request_id", activityRequestID(request), "operation_id", id, "action", subtitleOperationLogAction(action), "outcome", outcome)
}

func subtitleOperationLogAction(action string) string {
	switch action {
	case "apply":
		return "apply"
	case "restore":
		return "restore"
	case "fetch":
		return "fetch"
	case "replacement":
		return "replacement"
	case "audio":
		return "audio"
	case "maintain":
		return "maintain"
	case "fetch-wanted":
		return "fetch-wanted"
	default:
		return subtitleOperationLogRequest(action)
	}
}

func subtitleOperationLogRequest(action string) string {
	switch action {
	case "prepare":
		return "prepare"
	case "status":
		return "status"
	case "result":
		return "result"
	default:
		return "unknown"
	}
}
