package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

const privateResponseLimit = 65536

type restoreControlResponse struct { status int; header http.Header; body []byte }
type restoreCapture struct { header http.Header; status int; body bytes.Buffer; overflow bool }

func (capture *restoreCapture) Header() http.Header { return capture.header }
func (capture *restoreCapture) WriteHeader(status int) { if capture.status == 0 { capture.status = status } }
func (capture *restoreCapture) Write(data []byte) (int, error) {
	if capture.status == 0 { capture.status = http.StatusOK }
	if capture.body.Len()+len(data) > privateResponseLimit {
		capture.overflow = true
		return 0, errors.New("Restore private response bound exceeded")
	}
	return capture.body.Write(data)
}
func (f *restoreRig) capture(request *http.Request) *restoreCapture {
	response := &restoreCapture{header: make(http.Header)}
	f.app.ServeHTTP(response, request)
	if response.status == 0 { response.status = http.StatusOK }
	if response.overflow { f.failBoundary() }
	return response
}
func copyRestoreHeaders(writer http.ResponseWriter, response *restoreCapture) {
	for name, values := range response.header { writer.Header()[name] = append([]string(nil), values...) }
}
func writeRestoreActual(writer http.ResponseWriter, response *restoreCapture) bool {
	copyRestoreHeaders(writer, response)
	writer.WriteHeader(response.status)
	written, err := writer.Write(response.body.Bytes())
	return err == nil && written == response.body.Len()
}
func readPrivateResponse(response *http.Response) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
	return data, errors.Join(readErr, response.Body.Close())
}
func (f *restoreRig) closeFailedResponse(response *http.Response) {
	if response != nil { if err := response.Body.Close(); err != nil { f.failBoundary() } }
}
func restoreRequestBody(request *http.Request) ([]byte, bool) {
	data, readErr := io.ReadAll(io.LimitReader(request.Body, privateResponseLimit+1))
	err := errors.Join(readErr, request.Body.Close())
	if err != nil || len(data) > privateResponseLimit { return nil, false }
	request.Body = io.NopCloser(bytes.NewReader(data))
	return data, true
}
func (f *restoreRig) actualRestoreHeaders() http.Header {
	f.mu.Lock(); defer f.mu.Unlock()
	return f.restoreHeaders.Clone()
}
func (f *restoreRig) actualInspectionHeaders() http.Header {
	f.mu.Lock(); defer f.mu.Unlock()
	return f.inspectionHeaders.Clone()
}
func (f *restoreRig) actualHeldBody(inspection bool) []byte {
	f.mu.Lock(); defer f.mu.Unlock()
	if inspection { return append([]byte(nil), f.inspectionBody...) }
	return append([]byte(nil), f.restoreBody...)
}
func (f *restoreRig) servePreparation(target *restoreTarget, writer http.ResponseWriter, request *http.Request) {
	data, valid := restoreRequestBody(request)
	var input struct { Action, Item string }
	if !valid || json.Unmarshal(data, &input) != nil || input.Action != "restore" || !target.ownsItem(input.Item) {
		http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed); return
	}
	f.mu.Lock()
	f.state.PrepareAttempts++
	allowed := f.state.PrepareAttempts == 1 && f.state.RestoreAttempts == 0
	f.mu.Unlock()
	if !allowed { http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed); return }
	response := f.capture(request)
	var receipt restorePublicReceipt
	if response.status == http.StatusCreated {
		if response.overflow || json.Unmarshal(response.body.Bytes(), &receipt) != nil ||
			!validPreparedRestore(receipt, input.Item) || !target.registerReceipt(receipt) { f.failBoundary() }
	}
	if !writeRestoreActual(writer, response) && request.Context().Err() == nil { f.failBoundary() }
}

func proveRestoreHeadersPending(t *testing.T, target *restoreTarget, f *restoreRig, exchange *restoreExchange) restoreControlResponse {
	t.Helper()
	select {
	case <-exchange.response: t.Fatal("Restore held headers escaped before release")
	case <-exchange.failed: t.Fatal("Restore held request failed")
	case <-time.After(200*time.Millisecond):
	}
	released := restoreControlGET(t, target, f, "/__r06_restore/release")
	if released.status != http.StatusNoContent { t.Fatal("Restore public release unavailable") }
	var response *http.Response
	select {
	case response = <-exchange.response:
	case <-exchange.failed: t.Fatal("Restore released response failed")
	case <-time.After(3*time.Second): t.Fatal("Restore released response did not settle")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
	exchange.body = append([]byte(nil), data...)
	exchange.finishRead()
	if err != nil || !bytes.Equal(data, f.actualHeldBody(false)) { t.Fatal("Restore released actual body changed") }
	return exchange.receipt(t, response)
}
func proveRestoreInspectionBodyPending(t *testing.T, target *restoreTarget, f *restoreRig, exchange *restoreExchange) restoreControlResponse {
	t.Helper()
	var response *http.Response
	select {
	case response = <-exchange.response:
	case <-exchange.failed: t.Fatal("Restore inspection headers unavailable")
	case <-time.After(2*time.Second): t.Fatal("Restore body fault also withheld inspection headers")
	}
	done, started := exchange.readBody(response)
	<-started
	select {
	case <-done: t.Fatal("Restore held inspection body escaped before release")
	case <-time.After(200*time.Millisecond):
	}
	released := restoreControlGET(t, target, f, "/__r06_restore/release")
	if released.status != http.StatusNoContent { t.Fatal("Restore public release unavailable") }
	select {
	case exact := <-done: if !exact { t.Fatal("Restore released actual inspection body changed") }
	case <-time.After(3*time.Second): t.Fatal("Restore released inspection body did not settle")
	}
	return exchange.receipt(t, response)
}
