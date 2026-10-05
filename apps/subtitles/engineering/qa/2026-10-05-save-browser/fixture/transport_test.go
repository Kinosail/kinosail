package main

import (
 "bytes"
 "encoding/json"
 "errors"
 "io"
 "net/http"
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
 if capture.status == 0 { capture.status = http.StatusOK }
 if capture.body.Len()+len(data) > privateResponseLimit {
  capture.overflow = true
  return 0, errors.New("private response bound exceeded")
 }
 return capture.body.Write(data)
}

func (f *fixture) capture(request *http.Request) *capturedResponse {
 response := &capturedResponse{header: make(http.Header)}
 f.app.ServeHTTP(response, request)
 if response.status == 0 { response.status = http.StatusOK }
 if response.overflow { f.failBoundary() }
 return response
}

func writeActual(writer http.ResponseWriter, response *capturedResponse) bool {
 copyActualHeaders(writer, response)
 writer.WriteHeader(response.status)
 written, err := writer.Write(response.body.Bytes())
 return err == nil && written == response.body.Len()
}

func copyActualHeaders(writer http.ResponseWriter, response *capturedResponse) {
 for name, values := range response.header { writer.Header()[name] = append([]string(nil), values...) }
}

func (f *fixture) actualHeaders() http.Header {
 f.mu.Lock()
 defer f.mu.Unlock()
 return f.responseHeader.Clone()
}

func readPrivateBody(request *http.Request) ([]byte, bool) {
 data, readErr := io.ReadAll(io.LimitReader(request.Body, privateResponseLimit+1))
 err := errors.Join(readErr, request.Body.Close())
 if err != nil || len(data) > privateResponseLimit { return nil, false }
 request.Body = io.NopCloser(bytes.NewReader(data))
 return data, true
}

func (f *fixture) safePreview(request *http.Request) bool {
 data, valid := readPrivateBody(request)
 var input struct { Language, Text string; AutomaticSync bool }
 return valid && json.Unmarshal(data, &input) == nil && input.Language == "en" && input.Text == savedSRT && !input.AutomaticSync
}

func (f *fixture) servePreview(writer http.ResponseWriter, request *http.Request) {
 if !f.safePreview(request) {
  http.Error(writer, "Save-only fixture boundary", http.StatusMethodNotAllowed)
  return
 }
 f.app.ServeHTTP(writer, request)
}

func (f *fixture) prepare(writer http.ResponseWriter, request *http.Request) {
 allowed := f.admitPreparation()
 item, valid := preparationItem(request)
 if !allowed || !valid {
  http.Error(writer, "Save-only fixture boundary", http.StatusMethodNotAllowed)
  return
 }
 response := f.capture(request)
 f.recordPreparation(response, item)
 writeActual(writer, response)
}

func (f *fixture) admitPreparation() bool {
 f.mu.Lock()
 defer f.mu.Unlock()
 f.state.PrepareAttempts++
 return f.state.PrepareAttempts == 1 && f.state.SaveAttempts == 0
}

func preparationItem(request *http.Request) (string, bool) {
 data, valid := readPrivateBody(request)
 var input struct { Action, Item string }
 if !valid || json.Unmarshal(data, &input) != nil { return "", false }
 return input.Item, input.Action == "apply" && itemID.MatchString(input.Item)
}

func (f *fixture) recordPreparation(response *capturedResponse, item string) {
 var receipt publicReceipt
 if response.status != http.StatusCreated || json.Unmarshal(response.body.Bytes(), &receipt) != nil { return }
 if !operationID.MatchString(receipt.ID) || receipt.Action != "apply" || receipt.Item != item || receipt.State != "prepared" { return }
 f.mu.Lock()
 f.preparedID, f.preparedItem = receipt.ID, receipt.Item
 f.mu.Unlock()
}
