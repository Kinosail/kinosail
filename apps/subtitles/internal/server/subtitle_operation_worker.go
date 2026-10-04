package server

import (
	"context"
	"log/slog"
	"maps"
	"net/http"
	"time"
)

func (operations *subtitleOperations) activate(request *http.Request, activation subtitleActivation, work http.HandlerFunc) (subtitleOperationReceipt, int) {
	operations.mu.Lock()
	defer operations.mu.Unlock()
	record, status := operations.record(request, activation.id)
	if status != http.StatusOK {
		return record.subtitleOperationReceipt, status
	}
	if record.State == "unknown" || record.Digest != "" && record.Digest != activation.digest {
		return record.subtitleOperationReceipt, http.StatusConflict
	}
	if record.State != "prepared" {
		return record.subtitleOperationReceipt, http.StatusAccepted
	}
	if operations.lifecycle.Err() != nil {
		return record.subtitleOperationReceipt, http.StatusServiceUnavailable
	}
	if !operations.admission.claim() {
		return record.subtitleOperationReceipt, http.StatusConflict
	}
	limit := subtitleOperationDeadline(record.Action)
	record.State, record.Digest, record.Expires = "running", activation.digest, 0
	record.Deadline = operations.clock.Now().Add(limit).Unix()
	operations.records[record.ID] = record
	if operations.persist() != nil {
		operations.markUnknown(record)
		operations.admission.release()
		operations.persistenceFailed(request, record.Action, record.ID, "activation-not-started")
		return operations.records[record.ID].subtitleOperationReceipt, http.StatusServiceUnavailable
	}
	operations.launch(request, record, limit, work)
	return record.subtitleOperationReceipt, http.StatusAccepted
}

func (operations *subtitleOperations) launch(request *http.Request, record subtitleOperationRecord, limit time.Duration, work http.HandlerFunc) {
	// Browser cancellation does not undo an activation. The job keeps request
	// identity, follows Server shutdown, and has the reviewed outer deadline.
	ctx, cancel := operations.operationContext(request.Context(), limit)
	maps.Copy(operations.active, map[string]context.Context{record.ID: ctx})
	jobRequest := request.Clone(ctx)
	jobRequest.Body = http.NoBody
	slog.Info("subtitle operation started", "request_id", activityRequestID(request), "operation_id", record.ID, "action", record.Action, "outcome", "running")
	go func() {
		defer cancel()
		capture := &subtitleOperationResponse{header: http.Header{}, retain: record.Action == "audio"}
		work(capture, jobRequest)
		operations.complete(jobRequest, record, capture)
	}()
}

func (operations *subtitleOperations) operationContext(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	base, cancelBase := context.WithCancel(context.WithoutCancel(parent))
	stopLifecycle := context.AfterFunc(operations.lifecycle, cancelBase)
	limited, cancelLimit := operations.clock.WithTimeout(base, limit)
	admitted := context.WithValue(limited, subtitleAdmissionKey{}, operations.admission)
	return admitted, func() {
		stopLifecycle()
		cancelLimit()
		cancelBase()
	}
}

func (operations *subtitleOperations) markUnknown(record subtitleOperationRecord) {
	now := operations.clock.Now().Unix()
	record.State, record.Outcome, record.Status = "unknown", "uncertain", 0
	record.Completed, record.Expires = now, now+int64((30*time.Minute)/time.Second)
	operations.records[record.ID] = record
}

func (operations *subtitleOperations) complete(request *http.Request, record subtitleOperationRecord, response *subtitleOperationResponse) {
	operations.mu.Lock()
	defer operations.mu.Unlock()
	defer operations.admission.release()
	delete(operations.active, record.ID)
	now := operations.clock.Now().Unix()
	record.State, record.Status, record.Outcome = "completed", response.statusCode(), "failed"
	record.Completed, record.Expires = now, now+int64((30*time.Minute)/time.Second)
	if record.Status >= 200 && record.Status < 300 {
		record.Outcome = "success"
	}
	if record.Action == "audio" && record.Outcome == "success" && (response.overflow || !validSubtitleOperationAudio(response.body)) {
		record.Status, record.Outcome = http.StatusInternalServerError, "failed"
	}
	operations.records[record.ID] = record
	if operations.persist() != nil {
		operations.markUnknown(record)
		operations.persistenceFailed(request, record.Action, record.ID, "completion-uncertain")
		return
	}
	if record.Action == "audio" && record.Outcome == "success" {
		operations.retainResult(record.ID, response.body)
	}
	slog.Info("subtitle operation completed", "request_id", activityRequestID(request), "operation_id", record.ID, "action", record.Action, "status", record.Status, "outcome", record.Outcome)
}

type subtitleOperationResponse struct {
	header   http.Header
	status   int
	retain   bool
	overflow bool
	body     []byte
}

func (response *subtitleOperationResponse) Header() http.Header { return response.header }

func (response *subtitleOperationResponse) WriteHeader(status int) {
	if response.status == 0 {
		response.status = status
	}
}

func (response *subtitleOperationResponse) Write(data []byte) (int, error) {
	response.WriteHeader(http.StatusOK)
	if response.retain && !response.overflow {
		if len(response.body)+len(data) > 128*1024 {
			response.overflow, response.body = true, nil
		} else {
			response.body = append(response.body, data...)
		}
	}
	return len(data), nil
}

func (response *subtitleOperationResponse) statusCode() int {
	if response.status == 0 {
		return http.StatusOK
	}
	return response.status
}
