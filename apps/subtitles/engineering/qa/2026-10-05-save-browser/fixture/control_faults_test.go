package main

import (
	"bytes"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// Only copied response metadata leaves a transport proof. The
// HTTP body remains exclusively owned and closed by ownedSave.send.
type responseReceipt struct {
	status int
	header http.Header
}

func proveHeldHeaders(t *testing.T, f *fixture, exchange *ownedSave) responseReceipt {
	t.Helper()
	select {
	case <-exchange.response:
		t.Fatal("held headers escaped before release")
	case <-exchange.failed:
		t.Fatal("held request failed")
	case <-time.After(200 * time.Millisecond):
	}
	f.release()
	var response *http.Response
	select {
	case response = <-exchange.response:
	case <-exchange.failed:
		t.Fatal("released response failed")
	case <-time.After(3 * time.Second):
		t.Fatal("released response did not settle")
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
	exchange.finishRead()
	if readErr != nil || !bytes.Equal(data, f.actualBody()) {
		t.Fatal("released actual body changed")
	}
	return exchange.copiedReceipt(t, response)
}

func proveHeldBody(t *testing.T, f *fixture, exchange *ownedSave) responseReceipt {
	t.Helper()
	var response *http.Response
	select {
	case response = <-exchange.response:
	case <-exchange.failed:
		t.Fatal("actual response headers unavailable")
	case <-time.After(2 * time.Second):
		t.Fatal("body fault also withheld headers")
	}
	bodyDone, bodyStarted := exchange.readBody(response, f)
	<-bodyStarted
	select {
	case <-bodyDone:
		t.Fatal("held body escaped before release")
	case <-time.After(200 * time.Millisecond):
	}
	f.release()
	select {
	case exact := <-bodyDone:
		if !exact {
			t.Fatal("released actual body changed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("released body did not settle")
	}
	return exchange.copiedReceipt(t, response)
}

func (exchange *ownedSave) readBody(response *http.Response, f *fixture) (<-chan bool, <-chan struct{}) {
	done := make(chan bool, 1)
	started := make(chan struct{})
	exchange.bodyReadDone = make(chan struct{})
	go func() {
		defer close(exchange.bodyReadDone)
		defer exchange.finishRead()
		close(started)
		data, readErr := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
		done <- readErr == nil && bytes.Equal(data, f.actualBody())
	}()
	return done, started
}

func equalApplicationHeaders(received, actual http.Header) bool {
	for name, values := range actual {
		if !reflect.DeepEqual(received.Values(name), values) {
			return false
		}
	}
	for name := range received {
		if _, exists := actual[name]; exists {
			continue
		}
		if !transportHeader(name) {
			return false
		}
	}
	return true
}

func transportHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Date", "Content-Length", "Transfer-Encoding", "Connection":
		return true
	default:
		return false
	}
}

func (exchange *ownedSave) copiedReceipt(t *testing.T, response *http.Response) responseReceipt {
	t.Helper()
	select {
	case <-exchange.requestDone:
	case <-time.After(3 * time.Second):
		t.Fatal("owned request did not settle")
	}
	if exchange.bodyReadDone != nil {
		select {
		case <-exchange.bodyReadDone:
		case <-time.After(3 * time.Second):
			t.Fatal("owned body read did not settle")
		}
	}
	return responseReceipt{status: response.StatusCode, header: response.Header.Clone()}
}
