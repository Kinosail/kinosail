package server

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
)

func registerWeb(mux *http.ServeMux, authentication *auth.Manager) {
	mux.HandleFunc("GET /setup", func(writer http.ResponseWriter, request *http.Request) {
		if authentication.Configured() {
			http.Redirect(writer, request, "/", http.StatusSeeOther)
			return
		}
		serveHTML(writer, request, "web/setup.html")
	})
	mux.HandleFunc("GET /login", func(writer http.ResponseWriter, request *http.Request) {
		if !authentication.Configured() {
			http.Redirect(writer, request, "/setup", http.StatusSeeOther)
			return
		}
		if _, found := currentIdentity(request); found {
			http.Redirect(writer, request, "/", http.StatusSeeOther)
			return
		}
		serveHTML(writer, request, "web/login.html")
	})
	mux.HandleFunc("GET /{$}", authenticatedPage(authentication, "web/index.html"))
	mux.HandleFunc("GET /supporter", authenticatedPage(authentication, "web/supporter.html"))
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") || request.URL.Path == "/mcp" {
			http.NotFound(writer, request)
			return
		}
		serveHTMLStatus(writer, request, "web/not-found.html", http.StatusNotFound)
	})
}

func authenticatedPage(authentication *auth.Manager, path string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !authentication.Configured() {
			http.Redirect(writer, request, "/setup", http.StatusSeeOther)
			return
		}
		if _, found := currentIdentity(request); !found {
			http.Redirect(writer, request, "/login", http.StatusSeeOther)
			return
		}
		serveHTML(writer, request, path)
	}
}

func serveHTML(writer http.ResponseWriter, request *http.Request, path string) {
	serveHTMLStatus(writer, request, path, http.StatusOK)
}

func serveHTMLStatus(writer http.ResponseWriter, request *http.Request, path string, status int) {
	data, err := webAssets.ReadFile(path)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = writer.Write(data)
}
