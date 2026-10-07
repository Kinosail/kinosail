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

type restoreCompletionWitness struct {
	Matched  bool `json:"matched"`
	Bodyless bool `json:"bodyless"`
}

func (f *restoreRig) causalCompletionWitness(item string) restoreCompletionWitness {
	if f.target == nil || !f.target.ownsItem(item) {
		return restoreCompletionWitness{}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	matched := f.ctx.Err() == nil && f.restoreHeaders != nil &&
		f.state.Protocol == "legacy" && f.state.RestoreAttempts == 1 && f.state.ResponseStatus == http.StatusNoContent
	return restoreCompletionWitness{Matched: matched, Bodyless: matched && len(f.restoreBody) == 0}
}

func (f *restoreRig) serveCausalCompletion(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || !f.target.ownsItem(request.Header.Get("X-R06-Item")) {
		http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(f.causalCompletionWitness(request.Header.Get("X-R06-Item")))
}

func (f *restoreRig) causalSnapshot() [3]restoreProbeSnapshot {
	f.target.mu.Lock()
	defer f.target.mu.Unlock()
	return f.target.causalProbe
}

func (f *restoreRig) serveCausalProbe(writer http.ResponseWriter, request *http.Request) bool {
	if !causalProbePath(request.URL.Path) {
		return false
	}
	if request.URL.RawQuery != "" || request.URL.RawPath != "" || request.Host != f.target.authority {
		http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
		return true
	}
	if f.serveCausalRead(writer, request) {
		return true
	}
	f.serveCausalPOST(writer, request)
	return true
}

func causalProbePath(path string) bool {
	switch path {
	case "/__r06_restore/probe", "/__r06_restore/probe-witness", "/__r06_restore/completion-witness":
		return true
	default:
		return false
	}
}

func (f *restoreRig) serveCausalRead(writer http.ResponseWriter, request *http.Request) bool {
	switch request.URL.Path {
	case "/__r06_restore/completion-witness":
		f.serveCausalCompletion(writer, request)
	case "/__r06_restore/probe-witness":
		if request.Method != http.MethodGet {
			http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
			return true
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(writer).Encode(f.causalSnapshot())
	default:
		return false
	}
	return true
}

func (f *restoreRig) serveCausalPOST(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
		http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
		return
	}
	index := causalProbeIndex(request)
	if index < 0 {
		http.Error(writer, "fixed diagnostic boundary", http.StatusMethodNotAllowed)
		return
	}
	if !f.armCausalProbe(index) {
		http.Error(writer, "fixed diagnostic boundary", http.StatusConflict)
		return
	}
	if index == 2 {
		f.serveHeldCausalProbe(request.Context())
		panic(http.ErrAbortHandler) // A held diagnostic never delivers an implicit HTTP response.
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
}

func causalProbeIndex(request *http.Request) int {
	raw, err := io.ReadAll(io.LimitReader(request.Body, 257))
	closeErr := request.Body.Close()
	if err != nil || closeErr != nil || len(raw) > 256 {
		return -1
	}
	return causalProbeMode(raw)
}

func causalProbeMode(raw []byte) int {
	var input struct {
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
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
