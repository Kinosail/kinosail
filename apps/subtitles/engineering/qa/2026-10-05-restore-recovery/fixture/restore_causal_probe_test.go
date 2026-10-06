package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type restoreProbeSnapshot struct {
	Seen      bool `json:"seen"`
	Held      bool `json:"held"`
	Delivered bool `json:"delivered"`
	Cancelled bool `json:"cancelled"`
	TimedOut  bool `json:"timedOut"`
	Settled   bool `json:"settled"`
}

func (f *restoreRig) causalSnapshot() [3]restoreProbeSnapshot {
	f.target.mu.Lock()
	defer f.target.mu.Unlock()
	return f.target.causalProbe
}

func (f *restoreRig) serveCausalProbe(writer http.ResponseWriter, request *http.Request) bool {
	if request.URL.Path != "/__r06_restore/probe" && request.URL.Path != "/__r06_restore/probe-witness" {
		return false
	}
	if request.URL.RawQuery != "" || request.URL.RawPath != "" || request.Host != f.target.authority {
		http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
		return true
	}
	if request.URL.Path == "/__r06_restore/probe-witness" {
		if request.Method != http.MethodGet {
			http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
			return true
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(writer).Encode(f.causalSnapshot())
		return true
	}
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
		http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
		return true
	}
	index := causalProbeIndex(request)
	if index < 0 {
		http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
		return true
	}
	if !f.armCausalProbe(index) {
		http.Error(writer, "fixed diagnostic boundary", http.StatusConflict)
		return true
	}
	if index == 2 {
		f.serveHeldCausalProbe(request.Context())
		return true
	}
	delivered := true
	if index == 0 {
		causal204(writer)
	} else {
		captured := &restoreCapture{header: make(http.Header)}
		causal204(captured)
		delivered = writeRestoreActual(writer, captured)
	}
	f.target.mu.Lock()
	f.target.causalProbe[index].Delivered = delivered
	f.target.causalProbe[index].Settled = true
	f.target.mu.Unlock()
	return true
}

func causalProbeIndex(request *http.Request) int {
	raw, err := io.ReadAll(io.LimitReader(request.Body, 257))
	closeErr := request.Body.Close()
	var input struct {
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err != nil || closeErr != nil || len(raw) > 256 || decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return -1
	}
	index := -1
	switch input.Mode {
	case "direct":
		index = 0
	case "captured":
		index = 1
	case "held":
		index = 2
	}
	if index < 0 || !bytes.Equal(raw, []byte(`{"mode":"`+input.Mode+`"}`)) {
		return -1
	}
	return index
}

func (f *restoreRig) armCausalProbe(index int) bool {
	f.target.mu.Lock()
	defer f.target.mu.Unlock()
	allowed := !f.target.causalProbe[index].Seen
	if allowed {
		f.target.causalProbe[index].Seen = true
		f.target.causalProbe[index].Held = index == 2
	}
	return allowed
}

func (f *restoreRig) serveHeldCausalProbe(requestContext context.Context) {
	deadline := time.Now().Add(5 * time.Second)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	cancelled := false
	select {
	case <-requestContext.Done():
		cancelled = f.causalCancellationQualifies(deadline)
	case <-timer.C:
	case <-f.ctx.Done():
	}
	// The cancellation witness is taken inside this live handler, before return.
	f.target.mu.Lock()
	f.target.causalProbe[2].Cancelled = cancelled
	f.target.causalProbe[2].TimedOut = !cancelled
	f.target.causalProbe[2].Settled = true
	f.target.mu.Unlock()
}

func causal204(writer http.ResponseWriter) {
	// Fixed secret-free producer, identical through direct and actual capture paths.
	writer.Header().Set("Date", "Mon, 01 Jan 1990 00:00:00 GMT")
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (f *restoreRig) causalCancellationQualifies(deadline time.Time) bool {
	return f.ctx.Err() == nil && time.Now().Before(deadline)
}
