package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

type restoreExchange struct {
	rig *restoreRig
	inspection bool
	response chan *http.Response
	failed chan bool
	requestDone, bodyReadDone, consumed chan struct{}
	consumeOnce sync.Once
	cancel context.CancelFunc
	body []byte
}

func beginHeldRestore(t *testing.T, target *restoreTarget, f *restoreRig) *restoreExchange {
	t.Helper()
	return beginRestoreExchange(t, target, f, false)
}
func beginHeldRestoreInspection(t *testing.T, target *restoreTarget, f *restoreRig) *restoreExchange {
	t.Helper()
	return beginRestoreExchange(t, target, f, true)
}
func beginRestoreExchange(t *testing.T, target *restoreTarget, f *restoreRig, inspection bool) *restoreExchange {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	exchange := &restoreExchange{rig: f, inspection: inspection, response: make(chan *http.Response, 1),
		failed: make(chan bool, 1), requestDone: make(chan struct{}), consumed: make(chan struct{}), cancel: cancel}
	f.owned.Add(1)
	go exchange.send(t, ctx, target)
	return exchange
}
func (exchange *restoreExchange) send(t *testing.T, ctx context.Context, target *restoreTarget) {
	defer exchange.rig.owned.Done()
	defer close(exchange.requestDone)
	stop := exchange.rig.cancelOnLifecycle(exchange.cancel)
	defer stop()
	method, path, body := http.MethodPost, "/api/v1/subtitle-library/"+exchange.rig.item+"/restore", []byte(`{"language":"en"}`)
	if exchange.inspection { method, path, body = http.MethodGet, exchange.rig.inspectPath(), nil }
	endpoint, err := target.endpoint(path)
	if err != nil { exchange.failed <- true; return }
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil { exchange.failed <- true; return }
	client, csrf := target.controlOwner()
	if client == nil || csrf == "" { exchange.failed <- true; return }
	request.Header.Set("Origin", target.origin)
	request.Header.Set("User-Agent", "R06-restore-private-held")
	request.Header.Set("X-Kinosail-CSRF", csrf)
	setRestoreWriteHeaders(request, body, "")
	if !target.admittedRequest(request) { exchange.failed <- true; return }
	response, err := client.Do(request)
	if err != nil { if response != nil { closeRestoreControlBody(t, response) }; exchange.failed <- true; return }
	defer func() { closeRestoreControlBody(t, response) }()
	exchange.response <- response
	select { case <-exchange.consumed: case <-ctx.Done(): }
}
func closeRestoreControlBody(t *testing.T, response *http.Response) {
	if err := response.Body.Close(); err != nil { t.Error("Restore owned response body close failed") }
}
func (exchange *restoreExchange) finishRead() { exchange.consumeOnce.Do(func() { close(exchange.consumed) }) }
func (exchange *restoreExchange) readBody(response *http.Response) (<-chan bool, <-chan struct{}) {
	done, started := make(chan bool, 1), make(chan struct{})
	exchange.bodyReadDone = make(chan struct{})
	exchange.rig.owned.Add(1)
	go func() {
		defer exchange.rig.owned.Done()
		defer close(exchange.bodyReadDone)
		defer exchange.finishRead()
		close(started)
		data, err := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
		exchange.body = append([]byte(nil), data...)
		done <- err == nil && bytes.Equal(data, exchange.rig.actualHeldBody(exchange.inspection))
	}()
	return done, started
}
func (exchange *restoreExchange) receipt(t *testing.T, response *http.Response) restoreControlResponse {
	t.Helper()
	if !joinRestoreChannel(t, exchange.requestDone, "Restore owned request did not settle") { return restoreControlResponse{} }
	if exchange.bodyReadDone != nil && !joinRestoreChannel(t, exchange.bodyReadDone, "Restore owned body reader did not settle") { return restoreControlResponse{} }
	return restoreControlResponse{status: response.StatusCode, header: response.Header.Clone(), body: append([]byte(nil), exchange.body...)}
}
func joinRestoreChannel(t *testing.T, channel <-chan struct{}, message string) bool {
	t.Helper()
	select {
	case <-channel: return true
	case <-time.After(3*time.Second): t.Error(message); return false
	}
}
func (exchange *restoreExchange) stop(t *testing.T) {
	t.Helper()
	exchange.cancel()
	exchange.rig.release()
	joinRestoreChannel(t, exchange.requestDone, "Restore owned request did not settle")
	if exchange.bodyReadDone != nil { joinRestoreChannel(t, exchange.bodyReadDone, "Restore owned body reader did not settle") }
	if !exchange.rig.stop() { t.Error("Restore owned fixture descriptor did not close") }
}
