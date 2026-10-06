package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

type ownedSave struct {
	response     chan *http.Response
	failed       chan bool
	requestDone  chan struct{}
	bodyReadDone chan struct{}
	consumed     chan struct{}
	consumeOnce  sync.Once
	cancel       context.CancelFunc
}

func startOwnedSave(t *testing.T, target *ownedTarget, client *http.Client, path string, input []byte, headers http.Header) *ownedSave {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	exchange := &ownedSave{
		response: make(chan *http.Response, 1), failed: make(chan bool, 1),
		requestDone: make(chan struct{}), consumed: make(chan struct{}), cancel: cancel,
	}
	go exchange.send(t, ctx, target, client, path, input, headers)
	return exchange
}

func (exchange *ownedSave) send(t *testing.T, ctx context.Context, target *ownedTarget, client *http.Client, path string, input []byte, headers http.Header) {
	defer close(exchange.requestDone)
	endpoint, err := target.endpoint(path)
	if err != nil {
		exchange.failed <- true
		return
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(input))
	if err != nil {
		exchange.failed <- true
		return
	}
	request.Header = headers.Clone()
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", target.origin)
	request.Header.Set("User-Agent", "R06-private-control")
	if !target.admittedRequest(request) {
		exchange.failed <- true
		return
	}
	response, err := client.Do(request)
	if err != nil {
		if response != nil {
			closeControlBody(t, response)
		}
		exchange.failed <- true
		return
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Error("owned response body close failed")
		}
	}()
	exchange.response <- response
	select {
	case <-exchange.consumed:
	case <-ctx.Done():
	}
}

func closeControlBody(t *testing.T, response *http.Response) {
	if err := response.Body.Close(); err != nil {
		t.Error("owned response body close failed")
	}
}

func (exchange *ownedSave) finishRead() {
	exchange.consumeOnce.Do(func() { close(exchange.consumed) })
}

func (exchange *ownedSave) stop(t *testing.T, f *fixture) {
	t.Helper()
	exchange.cancel()
	f.controlStop(t)
	select {
	case <-exchange.requestDone:
	case <-time.After(3 * time.Second):
		t.Error("owned request did not settle")
	}
	if exchange.bodyReadDone == nil {
		return
	}
	select {
	case <-exchange.bodyReadDone:
	case <-time.After(3 * time.Second):
		t.Error("owned body read did not settle")
	}
}

func (f *fixture) controlStop(t *testing.T) {
	t.Helper()
	if !f.stop() {
		t.Error("owned fixture descriptor did not close")
	}
}

type checkedListener struct {
	net.Listener
	closeOnce sync.Once
	closeErr  error
}

func (listener *checkedListener) Close() error {
	listener.closeOnce.Do(func() { listener.closeErr = listener.Listener.Close() })
	return listener.closeErr
}

func (target *ownedTarget) stop() bool {
	target.stopOnce.Do(func() {
		target.tls.CloseClientConnections()
		target.tls.Close()
		target.closeOK = target.listener.Close() == nil
	})
	return target.closeOK
}
