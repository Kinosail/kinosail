package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func (f *restoreRig) serveRestore(target *restoreTarget, writer http.ResponseWriter, request *http.Request, id string) {
	operation := request.Header.Get("X-Kinosail-Operation")
	data, valid := restoreRequestBody(request)
	var input struct{ Language string }
	if !valid || json.Unmarshal(data, &input) != nil || !target.ownsItem(id) ||
		!f.admitRestore(target, operation, input.Language) {
		http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed)
		return
	}
	response := f.capture(request)
	f.recordRestoreResponse(response, operation)
	if !nominalRestoreResponse(operation, response.status) {
		f.deliverRestore(writer, request, response)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	stop := f.cancelOnLifecycle(cancel)
	defer stop()
	defer cancel()
	if response.overflow || !f.witnessRestore(ctx, target, request, id, operation, response.status) {
		f.failBoundary()
		f.deliverRestore(writer, request, response)
		return
	}
	if f.mode != "headers" {
		f.deliverRestore(writer, request, response)
		return
	}
	f.enterHold()
	defer f.finishHold()
	if f.awaitRelease(request, false) {
		f.deliverRestore(writer, request, response)
	}
}

func (f *restoreRig) admitRestore(target *restoreTarget, operation, language string) bool {
	registered := target.receiptID()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.RestoreAttempts++
	if operation == "" {
		return language == "en" && f.state.RestoreAttempts == 1 && f.state.PrepareAttempts == 0
	}
	// The exact-ID replay and changed-body conflict controls reach the actual
	// Server. HTTP attempts are distinct from actual History/write effects.
	return operation == registered && registered != "" && (language == "en" || language == "fr")
}

func (f *restoreRig) recordRestoreResponse(response *restoreCapture, operation string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restoreHeaders, f.restoreBody = response.header.Clone(), append([]byte(nil), response.body.Bytes()...)
	f.state.ResponseStatus, f.state.Protocol = response.status, "legacy"
	if operation != "" {
		f.state.Protocol = "prepared"
	}
}

func (f *restoreRig) enterHold() {
	f.mu.Lock()
	f.state.HoldEligible = true
	f.state.ActiveHolds++
	f.mu.Unlock()
	f.eligibleOnce.Do(func() { close(f.eligible) })
}
func (f *restoreRig) finishHold() { f.mu.Lock(); f.state.ActiveHolds--; f.mu.Unlock() }
func (f *restoreRig) deliverRestore(writer http.ResponseWriter, request *http.Request, response *restoreCapture) {
	complete := writeRestoreActual(writer, response)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.HeadersReleased, f.state.BodyReleased, f.state.RestoreResponseDelivered = true, complete, complete
	if request.Context().Err() != nil {
		f.state.RestoreClientCancelled = true
	}
	if !complete && !f.state.RestoreClientCancelled {
		f.state.BoundaryFailed = true
	}
}

func (f *restoreRig) holdInspection(target *restoreTarget, writer http.ResponseWriter, request *http.Request, response *restoreCapture) {
	if !f.releaseInspectionHeaders(writer, response) {
		return
	}
	f.enterHold()
	defer f.finishHold()
	if !f.awaitRelease(request, true) {
		return
	}
	written, err := writer.Write(response.body.Bytes())
	complete := err == nil && written == response.body.Len()
	f.mu.Lock()
	f.state.BodyReleased, f.state.InspectionResponseDelivered = complete, complete
	if request.Context().Err() != nil {
		f.state.InspectionClientCancelled = true
	}
	if !complete && !f.state.InspectionClientCancelled {
		f.state.BoundaryFailed = true
	}
	f.mu.Unlock()
	if complete {
		f.recordBrowserRead(target, request, response)
	}
}

func (f *restoreRig) releaseInspectionHeaders(writer http.ResponseWriter, response *restoreCapture) bool {
	copyRestoreHeaders(writer, response)
	if writer.Header().Get("Content-Length") == "" {
		writer.Header().Set("Content-Length", strconv.Itoa(response.body.Len()))
	}
	writer.WriteHeader(response.status)
	flush, ok := writer.(http.Flusher)
	if !ok {
		f.failBoundary()
		return false
	}
	flush.Flush()
	f.mu.Lock()
	f.state.HeadersReleased = true
	f.state.BodyReleased = false
	f.mu.Unlock()
	return true
}

func (f *restoreRig) awaitRelease(request *http.Request, inspection bool) bool {
	timer := time.NewTimer(80 * time.Second)
	defer timer.Stop()
	select {
	case <-f.released:
	case <-request.Context().Done():
		f.markCancelled(inspection)
		return false
	case <-f.ctx.Done():
		return false
	case <-timer.C:
		f.mu.Lock()
		f.state.HoldExpired = true
		f.mu.Unlock()
		return false
	}
	if request.Context().Err() != nil {
		f.markCancelled(inspection)
		return false
	}
	return true
}

func (f *restoreRig) markCancelled(inspection bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if inspection {
		f.state.InspectionClientCancelled = true
	} else {
		f.state.RestoreClientCancelled = true
	}
}

func (f *restoreRig) cancelOnLifecycle(cancel context.CancelFunc) func() bool {
	return context.AfterFunc(f.ctx, cancel)
}
