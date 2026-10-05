package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (f *restoreRig) serve(target *restoreTarget, writer http.ResponseWriter, request *http.Request) {
	if f.servePrivate(writer, request) { return }
	if strings.HasPrefix(request.URL.Path, "/media/") || strings.HasPrefix(request.URL.Path, "/watch/") ||
		strings.HasPrefix(request.URL.Path, "/__r06_restore/") {
		http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed); return
	}
	if f.serveRead(target, writer, request) || f.serveOwner(writer, request) { return }
	if f.servePost(target, writer, request) { return }
	http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed)
}
func (f *restoreRig) servePrivate(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodGet { return false }
	switch request.URL.Path {
	case "/__r06_restore/witness":
		writer.Header().Set("Content-Type", "application/json"); writer.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(writer).Encode(f.snapshot()); err != nil && request.Context().Err() == nil { f.failBoundary() }
	case "/__r06_restore/release":
		f.release(); writer.WriteHeader(http.StatusNoContent)
	default: return false
	}
	return true
}
func (f *restoreRig) serveOwner(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodPost { return false }
	switch request.URL.Path {
	case "/setup", "/login", "/account/mfa/enable":
		f.app.ServeHTTP(writer, request); return true
	default: return false
	}
}
func (f *restoreRig) serveRead(target *restoreTarget, writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodGet && request.Method != http.MethodHead { return false }
	if restoreCatalogRequest(request) { f.serveCatalog(target, writer, request); return true }
	if strings.HasPrefix(request.URL.Path, "/api/v1/subtitle-operations/") || strings.HasSuffix(request.URL.Path, "/inspect") {
		f.serveObservedRead(target, writer, request); return true
	}
	f.app.ServeHTTP(writer, request)
	return true
}
func restoreCatalogRequest(request *http.Request) bool {
	return request.Method == http.MethodGet && request.URL.Path == "/api/v1/subtitle-library" && request.URL.Query().Get("view") == "library"
}
func (f *restoreRig) serveCatalog(target *restoreTarget, writer http.ResponseWriter, request *http.Request) {
	response := f.capture(request)
	if response.status == http.StatusOK && !target.registerCatalog(response) { f.failBoundary() }
	if !writeRestoreActual(writer, response) && request.Context().Err() == nil { f.failBoundary() }
}
func (f *restoreRig) serveObservedRead(target *restoreTarget, writer http.ResponseWriter, request *http.Request) {
	if !target.ownsPath(request.URL.RequestURI()) { http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed); return }
	response := f.capture(request)
	if strings.HasSuffix(request.URL.Path, "/inspect") && f.armInspection(request, response) {
		f.holdInspection(target, writer, request, response); return
	}
	if writeRestoreActual(writer, response) { f.recordBrowserRead(target, request, response) } else if request.Context().Err() == nil { f.failBoundary() }
}
func (f *restoreRig) servePost(target *restoreTarget, writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodPost { return false }
	if request.URL.Path == "/api/v1/subtitle-operations" { f.servePreparation(target, writer, request); return true }
	if !target.ownsPath(request.URL.RequestURI()) { return false }
	if strings.HasSuffix(request.URL.Path, "/apply") { f.serveSetup(target, writer, request); return true }
	match := restoreMutationPath.FindStringSubmatch(request.URL.Path)
	if len(match) == 2 { f.serveRestore(target, writer, request, match[1]); return true }
	return false
}
func (f *restoreRig) serveSetup(target *restoreTarget, writer http.ResponseWriter, request *http.Request) {
	data, valid := restoreRequestBody(request)
	var input restoreSetupInput
	if invalidRestoreSetupInput(data, valid, &input, request) {
		http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed); return
	}
	f.mu.Lock(); f.state.SetupSaveAttempts++; allowed := f.state.SetupSaveAttempts == 1 && f.state.RestoreAttempts == 0; f.mu.Unlock()
	if !allowed { http.Error(writer, "Restore-only fixture boundary", http.StatusMethodNotAllowed); return }
	response := f.capture(request)
	if response.status == http.StatusOK && !f.rememberSetup(target, request) { f.failBoundary() }
	if !writeRestoreActual(writer, response) && request.Context().Err() == nil { f.failBoundary() }
}
func (f *restoreRig) armInspection(request *http.Request, response *restoreCapture) bool {
	private := strings.HasPrefix(request.UserAgent(), "R06-restore-private-")
	if private && request.UserAgent() != "R06-restore-private-held" { return false }
	f.mu.Lock(); defer f.mu.Unlock()
	if f.mode != "inspect-body" || f.inspectionHeld || !f.state.ActualRestored || response.status != http.StatusOK || response.overflow { return false }
	f.inspectionHeld = true
	f.inspectionHeaders, f.inspectionBody = response.header.Clone(), append([]byte(nil), response.body.Bytes()...)
	f.state.InspectionResponseStatus = response.status
	return true
}

type restoreSetupInput struct {
	Language, Fingerprint, Text, Data, DraftID string
	OffsetMilliseconds int
	AutomaticSync, RemoveCredits, MergeRepeated bool
	Anchors []json.RawMessage
}
func (input restoreSetupInput) fixedOptions() bool {
	return input.Language == "en" && restoreOperationID.MatchString(input.Fingerprint) && input.OffsetMilliseconds == 500 &&
		!input.AutomaticSync && !input.RemoveCredits && !input.MergeRepeated
}
func (input restoreSetupInput) noExternalPayload() bool {
	return input.Text == "" && input.Data == "" && input.DraftID == "" && len(input.Anchors) == 0
}

func invalidRestoreSetupInput(data []byte, valid bool, input *restoreSetupInput, request *http.Request) bool {
	return !valid || json.Unmarshal(data, input) != nil || !input.fixedOptions() ||
		!input.noExternalPayload() || request.Header.Get("X-Kinosail-Operation") != ""
}
