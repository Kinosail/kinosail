package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const privateResponseLimit = 65536

type capturedResponse struct {
	header http.Header
	status int
	body bytes.Buffer
	overflow bool
}

func (capture *capturedResponse) Header() http.Header { return capture.header }
func (capture *capturedResponse) WriteHeader(status int) {
	if capture.status == 0 { capture.status = status }
}
func (capture *capturedResponse) Write(data []byte) (int, error) {
	if capture.status == 0 { capture.status = 200 }
	if capture.body.Len()+len(data) > privateResponseLimit {
		capture.overflow = true
		return 0, errors.New("private response bound exceeded")
	}
	return capture.body.Write(data)
}

func (f *fixture) capture(request *http.Request) *capturedResponse {
	response := &capturedResponse{header:make(http.Header)}
	f.app.ServeHTTP(response, request)
	if response.status == 0 { response.status = 200 }
	if response.overflow {
		f.mu.Lock()
		f.state.BoundaryFailed = true
		f.mu.Unlock()
	}
	return response
}

func writeActual(writer http.ResponseWriter, response *capturedResponse) bool {
	for name, values := range response.header {
		writer.Header()[name] = append([]string(nil), values...)
	}
	writer.WriteHeader(response.status)
	written, err := writer.Write(response.body.Bytes())
	return err == nil && written == response.body.Len()
}

func (f *fixture) actualHeaders() http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.responseHeader.Clone()
}

func (f *fixture) serve(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet && request.URL.Path == "/__r06/witness" {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(writer).Encode(f.snapshot())
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/__r06/release" {
		f.release()
		writer.WriteHeader(204)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/media/") ||
		strings.HasPrefix(request.URL.Path, "/watch/") ||
		strings.HasPrefix(request.URL.Path, "/__r06/") {
		http.Error(writer, "Save-only fixture boundary", 405)
		return
	}
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		if strings.Contains(request.URL.Path, "/subtitle-operations/") ||
			strings.HasSuffix(request.URL.Path, "/inspect") {
			response := f.capture(request)
			f.recordBrowserRead(request, response)
			writeActual(writer, response)
			return
		}
		f.app.ServeHTTP(writer, request)
		return
	}
	if request.Method == http.MethodPost &&
		(request.URL.Path == "/setup" || request.URL.Path == "/login" ||
		request.URL.Path == "/account/mfa/enable") {
		f.app.ServeHTTP(writer, request)
		return
	}
	if request.Method == http.MethodPost && request.URL.Path == "/api/v1/subtitle-operations" {
		f.prepare(writer, request)
		return
	}
	if request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/preview") {
		if !f.safePreview(request) { http.Error(writer, "Save-only fixture boundary", 405); return }
		f.app.ServeHTTP(writer, request)
		return
	}
	match := applyPath.FindStringSubmatch(request.URL.Path)
	if request.Method == http.MethodPost && len(match) == 2 {
		f.save(writer, request, match[1])
		return
	}
	http.Error(writer, "Save-only fixture boundary", 405)
}

func readPrivateBody(request *http.Request) ([]byte, bool) {
	data, err := io.ReadAll(io.LimitReader(request.Body, privateResponseLimit+1))
	request.Body.Close()
	if err != nil || len(data) > privateResponseLimit { return nil, false }
	request.Body = io.NopCloser(bytes.NewReader(data))
	return data, true
}

func (f *fixture) safePreview(request *http.Request) bool {
	data, valid := readPrivateBody(request)
	var input struct { Language, Text string; AutomaticSync bool }
	return valid && json.Unmarshal(data, &input) == nil && input.Language == "en" &&
		input.Text == savedSRT && !input.AutomaticSync
}

func (f *fixture) prepare(writer http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	f.state.PrepareAttempts++
	allowed := f.state.PrepareAttempts == 1 && f.state.SaveAttempts == 0
	f.mu.Unlock()
	data, valid := readPrivateBody(request)
	var input struct { Action, Item string }
	if !allowed || !valid || json.Unmarshal(data, &input) != nil ||
		input.Action != "apply" || !itemID.MatchString(input.Item) {
		http.Error(writer, "Save-only fixture boundary", 405)
		return
	}
	response := f.capture(request)
	var receipt struct { ID, Action, Item, State string }
	if response.status == 201 && json.Unmarshal(response.body.Bytes(), &receipt) == nil &&
		operationID.MatchString(receipt.ID) && receipt.Action == "apply" &&
		receipt.Item == input.Item && receipt.State == "prepared" {
		f.mu.Lock()
		f.preparedID, f.preparedItem = receipt.ID, receipt.Item
		f.mu.Unlock()
	}
	writeActual(writer, response)
}

func (f *fixture) save(writer http.ResponseWriter, request *http.Request, id string) {
	f.mu.Lock()
	f.state.SaveAttempts++
	header := request.Header.Get("X-Kinosail-Operation")
	allowed := f.state.SaveAttempts == 1 &&
		((f.state.PrepareAttempts == 0 && header == "") ||
		(header != "" && header == f.preparedID && id == f.preparedItem))
	f.mu.Unlock()
	if !allowed || !f.safePreview(request) {
		http.Error(writer, "Save-only fixture boundary", 405)
		return
	}
	response := f.capture(request)
	f.mu.Lock()
	f.responseBody = append([]byte(nil), response.body.Bytes()...)
	f.responseHeader = response.header.Clone()
	f.state.ResponseStatus = response.status
	f.state.Protocol = "legacy"
	if header != "" { f.state.Protocol = "prepared" }
	f.mu.Unlock()
	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()
	if response.overflow || !f.witness(ctx, request, id, header, response.status) {
		f.mu.Lock()
		f.state.BoundaryFailed = true
		f.mu.Unlock()
		writeActual(writer, response)
		return
	}
	f.mu.Lock()
	f.state.HoldEligible = true
	f.state.ActiveHolds++
	f.mu.Unlock()
	close(f.eligible)
	defer func() { f.mu.Lock(); f.state.ActiveHolds--; f.mu.Unlock() }()
	if f.mode == "body" {
		for name, values := range response.header { writer.Header()[name] = append([]string(nil), values...) }
		if writer.Header().Get("Content-Length") == "" { writer.Header().Set("Content-Length", strconv.Itoa(response.body.Len())) }
		writer.WriteHeader(response.status)
		if flush, ok := writer.(http.Flusher); ok { flush.Flush() } else {
			f.mu.Lock(); f.state.BoundaryFailed = true; f.mu.Unlock(); return
		}
		f.mu.Lock(); f.state.HeadersReleased = true; f.mu.Unlock()
	}
	timer := time.NewTimer(80*time.Second)
	defer timer.Stop()
	select {
	case <-f.released:
	case <-request.Context().Done():
		f.mu.Lock(); f.state.ClientCancelled = true; f.mu.Unlock(); return
	case <-f.ctx.Done(): return
	case <-timer.C:
		f.mu.Lock(); f.state.HoldExpired = true; f.mu.Unlock(); return
	}
	if request.Context().Err() != nil {
		f.mu.Lock(); f.state.ClientCancelled = true; f.mu.Unlock(); return
	}
	complete := false
	if f.mode == "headers" {
		complete = writeActual(writer, response)
	} else {
		written, err := writer.Write(response.body.Bytes())
		complete = err == nil && written == response.body.Len()
	}
	f.mu.Lock()
	f.state.HeadersReleased = true
	f.state.BodyReleased, f.state.ResponseBodyWritten = complete, complete
	if request.Context().Err() != nil { f.state.ClientCancelled = true }
	if !complete && !f.state.ClientCancelled { f.state.BoundaryFailed = true }
	f.mu.Unlock()
}
