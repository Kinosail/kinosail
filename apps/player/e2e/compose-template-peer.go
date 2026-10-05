//go:build q47proof

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"sync"
	"time"
)

const holdLimit = 25 * time.Second
var modes = map[string]bool{
	"valid": true, "headers": true, "body": true, "loss": true,
	"late_headers": true, "late_body": true, "http_error": true,
}
var apps = map[string]bool{"player": true, "subtitles": true, "both": true}
type control struct {
	Operation string `json:"operation"`
	Mode      string `json:"mode"`
	App       string `json:"app"`
}
type witness struct {
	Mode           string          `json:"mode"`
	App            string          `json:"app"`
	TemplateSHA256 string          `json:"templateSHA256"`
	Counts         map[string]int  `json:"counts"`
	Checks         map[string]bool `json:"checks"`
}
type peer struct {
	mu        sync.Mutex
	files     http.Handler
	templates map[string][]byte
	release   chan struct{}
	state     witness
}

func (p *peer) reset(mode, app string) {
	p.release = make(chan struct{})
	p.state = witness{Mode: mode, App: app,
		TemplateSHA256: fmt.Sprintf("%x", sha256.Sum256(p.templates[app])),
		Counts: map[string]int{"requests": 0, "active": 0, "holds": 0,
			"canceled": 0, "completed": 0, "failures": 0, "expired": 0, "templateBytes": len(p.templates[app])},
		Checks: map[string]bool{"cookieSeen": false, "authorizationSeen": false, "bodySeen": false,
			"querySeen": false, "bodyPrefixSent": false}}
}
func (p *peer) snapshot() witness {
	p.mu.Lock()
	defer p.mu.Unlock()
	copy := p.state
	copy.Counts = maps.Clone(copy.Counts)
	copy.Checks = maps.Clone(copy.Checks)
	return copy
}
func (p *peer) controller(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Header.Get("X-Q47-Control") != "1" {
		http.Error(w, "control rejected", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var next control
	if decoder.Decode(&next) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		!modes[next.Mode] || !apps[next.App] {
		http.Error(w, "control rejected", http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch next.Operation {
	case "start":
		if p.state.Counts["active"] != 0 {
			http.Error(w, "control busy", http.StatusConflict)
			return
		}
		p.reset(next.Mode, next.App)
	case "recover":
		p.state.Mode = "valid"
	case "release":
		select {
		case <-p.release:
		default:
			close(p.release)
		}
	default:
		http.Error(w, "control rejected", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func failEvery(mode string) bool {
	return mode == "loss" || mode == "http_error"
}
func (p *peer) begin(r *http.Request, app string) (string, <-chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.Counts["requests"]++
	p.state.Counts["active"]++
	p.state.Checks["cookieSeen"] = p.state.Checks["cookieSeen"] || r.Header.Get("Cookie") != ""
	p.state.Checks["authorizationSeen"] = p.state.Checks["authorizationSeen"] || r.Header.Get("Authorization") != ""
	p.state.Checks["bodySeen"] = p.state.Checks["bodySeen"] || r.ContentLength > 0 || len(r.TransferEncoding) > 0
	p.state.Checks["querySeen"] = p.state.Checks["querySeen"] || r.URL.RawQuery != ""
	mode := p.state.Mode
	if app != p.state.App || p.state.Counts["requests"] > 1 && !failEvery(mode) {
		mode = "valid"
	}
	return mode, p.release
}
func (p *peer) finish(outcome string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.Counts["active"]--
	key := map[string]string{"complete": "completed", "canceled": "canceled", "expired": "expired"}[outcome]
	if key == "" {
		key = "failures"
	}
	p.state.Counts[key]++
}
func (p *peer) hold(r *http.Request, release <-chan struct{}) string {
	p.mu.Lock()
	p.state.Counts["holds"]++
	p.mu.Unlock()
	timer := time.NewTimer(holdLimit)
	defer timer.Stop()
	select {
	case <-r.Context().Done():
		return "canceled"
	case <-release:
		return "released"
	case <-timer.C:
		return "expired"
	}
}
func (p *peer) sendPrefix(w http.ResponseWriter, data []byte) bool {
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	_, err := w.Write(data[:32])
	if err == nil {
		err = http.NewResponseController(w).Flush()
	}
	if err != nil {
		return false
	}
	p.mu.Lock()
	p.state.Checks["bodyPrefixSent"] = true
	p.mu.Unlock()
	return true
}
func (p *peer) template(w http.ResponseWriter, r *http.Request, app string) {
	mode, release := p.begin(r, app)
	outcome := "failed"
	defer func() { p.finish(outcome) }()
	data := p.templates[app]
	w.Header().Set("Content-Type", "application/yaml")
	switch mode {
	case "loss":
		panic(http.ErrAbortHandler)
	case "http_error":
		http.Error(w, "template unavailable", http.StatusServiceUnavailable)
		return
	case "body", "late_body":
		if !p.sendPrefix(w, data) {
			return
		}
	}
	if mode != "valid" {
		outcome = p.hold(r, release)
		if outcome != "released" {
			return
		}
	}
	if mode == "body" || mode == "late_body" {
		data = data[32:]
	}
	if _, err := w.Write(data); err == nil {
		outcome = "complete"
	}
}
func (p *peer) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/__q47__/control" {
		p.controller(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method rejected", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/__q47__/witness" {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(p.snapshot()); err != nil {
			return
		}
		return
	}
	for app := range apps {
		if r.URL.Path == "/assets/install/"+app+".yaml" {
			p.template(w, r, app)
			return
		}
	}
	p.files.ServeHTTP(w, r)
}
