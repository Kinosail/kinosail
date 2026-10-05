package main

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

func (f *fixture) save(writer http.ResponseWriter, request *http.Request, id string) {
	header := request.Header.Get("X-Kinosail-Operation")
	if !f.admitSave(id, header) || !f.safePreview(request) {
		http.Error(writer, "Save-only fixture boundary", http.StatusMethodNotAllowed)
		return
	}
	response := f.capture(request)
	f.recordSaveResponse(response, header)
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	stop := f.cancelOnLifecycle(cancel)
	defer stop()
	defer cancel()
	if response.overflow || !f.witness(ctx, request, id, header, response.status) {
		f.failBoundary()
		writeActual(writer, response)
		return
	}
	f.mu.Lock()
	f.state.HoldEligible = true
	f.state.ActiveHolds++
	f.mu.Unlock()
	close(f.eligible)
	defer f.finishHold()
	if f.mode == "body" && !f.releaseHeaders(writer, response) {
		return
	}
	if !f.awaitRelease(request) {
		return
	}
	f.deliverHeld(writer, request, response)
}

func (f *fixture) admitSave(id, header string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.SaveAttempts++
	legacy := f.state.PrepareAttempts == 0 && header == ""
	prepared := header != "" && header == f.preparedID && id == f.preparedItem
	return f.state.SaveAttempts == 1 && (legacy || prepared)
}

func (f *fixture) recordSaveResponse(response *capturedResponse, operation string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responseBody = append([]byte(nil), response.body.Bytes()...)
	f.responseHeader = response.header.Clone()
	f.state.ResponseStatus = response.status
	f.state.Protocol = "legacy"
	if operation != "" {
		f.state.Protocol = "prepared"
	}
}

func (f *fixture) finishHold() {
	f.mu.Lock()
	f.state.ActiveHolds--
	f.mu.Unlock()
}

func (f *fixture) releaseHeaders(writer http.ResponseWriter, response *capturedResponse) bool {
	copyActualHeaders(writer, response)
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
	f.mu.Unlock()
	return true
}

func (f *fixture) awaitRelease(request *http.Request) bool {
	timer := time.NewTimer(80 * time.Second)
	defer timer.Stop()
	select {
	case <-f.released:
	case <-request.Context().Done():
		f.markCancelled()
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
		f.markCancelled()
		return false
	}
	return true
}

func (f *fixture) markCancelled() {
	f.mu.Lock()
	f.state.ClientCancelled = true
	f.mu.Unlock()
}

func (f *fixture) deliverHeld(writer http.ResponseWriter, request *http.Request, response *capturedResponse) {
	var complete bool
	if f.mode == "headers" {
		complete = writeActual(writer, response)
	} else {
		written, err := writer.Write(response.body.Bytes())
		complete = err == nil && written == response.body.Len()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.HeadersReleased = true
	f.state.BodyReleased, f.state.ResponseBodyWritten = complete, complete
	if request.Context().Err() != nil {
		f.state.ClientCancelled = true
	}
	if !complete && !f.state.ClientCancelled {
		f.state.BoundaryFailed = true
	}
}

func (f *fixture) cancelOnLifecycle(cancel context.CancelFunc) func() bool {
	return context.AfterFunc(f.ctx, cancel)
}
