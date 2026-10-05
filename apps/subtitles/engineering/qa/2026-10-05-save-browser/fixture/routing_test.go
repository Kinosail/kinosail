package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (f *fixture) serve(writer http.ResponseWriter, request *http.Request) {
	if f.servePrivate(writer, request) {
		return
	}
	if deniedFixturePath(request.URL.Path) {
		http.Error(writer, "Save-only fixture boundary", http.StatusMethodNotAllowed)
		return
	}
	if f.serveRead(writer, request) || f.serveOwner(writer, request) {
		return
	}
	if f.serveSavePost(writer, request) {
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

func (f *fixture) serveRead(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		return false
	}
	if strings.Contains(request.URL.Path, "/subtitle-operations/") || strings.HasSuffix(request.URL.Path, "/inspect") {
		response := f.capture(request)
		f.recordBrowserRead(request, response)
		writeActual(writer, response)
	} else {
		f.app.ServeHTTP(writer, request)
	}
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

func (f *fixture) serveSavePost(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodPost {
		return false
	}
	if request.URL.Path == "/api/v1/subtitle-operations" {
		f.prepare(writer, request)
		return true
	}
	if strings.HasSuffix(request.URL.Path, "/preview") {
		f.servePreview(writer, request)
		return true
	}
	match := applyPath.FindStringSubmatch(request.URL.Path)
	if len(match) == 2 {
		f.save(writer, request, match[1])
		return true
	}
	return false
}
