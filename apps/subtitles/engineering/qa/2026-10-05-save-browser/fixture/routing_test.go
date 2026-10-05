package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (f *fixture) serve(target *ownedTarget, writer http.ResponseWriter, request *http.Request) {
	if f.servePrivate(writer, request) {
		return
	}
	if deniedFixturePath(request.URL.Path) {
		http.Error(writer, "Save-only fixture boundary", http.StatusMethodNotAllowed)
		return
	}
	if f.serveRead(target, writer, request) || f.serveOwner(writer, request) {
		return
	}
	if f.serveSavePost(target, writer, request) {
		return
	}

	http.Error(writer, "Save-only fixture boundary", http.StatusMethodNotAllowed)
}

func deniedFixturePath(path string) bool {
	return strings.HasPrefix(path, "/media/") || strings.HasPrefix(path, "/watch/") || strings.HasPrefix(path, "/__r06/")
}

func (f *fixture) servePrivate(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodGet {
		return false
	}
	switch request.URL.Path {
	case "/__r06/witness":
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(writer).Encode(f.snapshot()); err != nil && request.Context().Err() == nil {
			f.failBoundary()
		}
	case "/__r06/release":
		f.release()
		writer.WriteHeader(http.StatusNoContent)
	default:
		return false
	}
	return true
}

func (f *fixture) serveRead(target *ownedTarget, writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		return false
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/subtitle-library" && request.URL.Query().Get("view") == "library" {
		f.serveCatalog(target, writer, request)
		return true
	}
	if strings.Contains(request.URL.Path, "/subtitle-operations/") || strings.HasSuffix(request.URL.Path, "/inspect") {
		if !target.ownsPath(request.URL.RequestURI()) {
			http.Error(writer, "Save-only fixture boundary", http.StatusMethodNotAllowed)
			return true
		}
		response := f.capture(request)
		f.recordBrowserRead(request, response)
		writeActual(writer, response)
	} else {
		f.app.ServeHTTP(writer, request)
	}
	return true
}

func (f *fixture) serveCatalog(target *ownedTarget, writer http.ResponseWriter, request *http.Request) {
	response := f.capture(request)
	if response.status == http.StatusOK && !target.registerCatalog(response) {
		f.failBoundary()
	}
	writeActual(writer, response)
}

func (target *ownedTarget) registerCatalog(response *capturedResponse) bool {
	var catalog struct{ Items []struct{ ID string } }
	if response.status != http.StatusOK || response.overflow || response.body.Len() > privateResponseLimit || json.Unmarshal(response.body.Bytes(), &catalog) != nil {
		return false
	}
	if len(catalog.Items) != 1 || !itemID.MatchString(catalog.Items[0].ID) {
		return false
	}
	return target.registerItem(catalog.Items[0].ID)
}

func (target *ownedTarget) registerItem(id string) bool {
	if !itemID.MatchString(id) {
		return false
	}
	target.mu.Lock()
	defer target.mu.Unlock()
	if target.item != "" && target.item != id {
		return false
	}
	target.item = id
	base := "/api/v1/subtitle-library/" + id
	for _, suffix := range []string{"/inspect?language=en", "/export?language=en&format=srt", "/preview", "/apply"} {
		target.addRouteLocked(base + suffix)
	}
	return true
}

func (target *ownedTarget) registerReceipt(receipt publicReceipt) bool {
	target.mu.Lock()
	defer target.mu.Unlock()
	if target.item == "" || receipt.Item != target.item || receipt.Action != "apply" || receipt.State != "prepared" {
		return false
	}
	if !operationID.MatchString(receipt.ID) || target.receipt != "" && target.receipt != receipt.ID {
		return false
	}
	target.receipt = receipt.ID
	target.addRouteLocked("/api/v1/subtitle-operations/" + receipt.ID)
	return true
}

func (f *fixture) serveOwner(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodPost {
		return false
	}
	switch request.URL.Path {
	case "/setup", "/login", "/account/mfa/enable":
		f.app.ServeHTTP(writer, request)
		return true
	default:
		return false
	}
}

func (f *fixture) serveSavePost(target *ownedTarget, writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodPost {
		return false
	}
	if request.URL.Path == "/api/v1/subtitle-operations" {
		f.prepare(target, writer, request)
		return true
	}
	if !target.ownsPath(request.URL.RequestURI()) {
		return false
	}
	if strings.HasSuffix(request.URL.Path, "/preview") {
		f.servePreview(writer, request)
		return true
	}
	match := applyPath.FindStringSubmatch(request.URL.Path)
	if len(match) == 2 {
		f.save(target, writer, request, match[1])
		return true
	}
	return false
}
