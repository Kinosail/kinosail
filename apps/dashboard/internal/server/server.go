// Package server provides Kinosail Dashboard's web and JSON adapters.
package server

import (
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
	"github.com/MikeO7/kinosail-dashboard/internal/supporter"
	"github.com/MikeO7/kinosail/packages/webassets"
)

//go:embed web/*
var webAssets embed.FS

type identityKey struct{}

// Config contains transport-specific behavior.
type Config struct {
	SecureCookies bool
	TrustedHosts  []string
	PublicURL     string
}

// New registers public, protected, API, and static routes.
func New(config Config, board *dashboard.Service, authentication *auth.Manager, prober *dashboard.Prober, programs ...*supporter.Service) http.Handler {
	program, _ := supporter.New(context.Background(), nil, supporter.Config{})
	if len(programs) == 1 && programs[0] != nil {
		program = programs[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]string{"status": "ok"}, http.StatusOK)
	})
	mux.HandleFunc("GET /static/{name}", serveAsset("web/static/"))
	mux.HandleFunc("GET /static/manrope.woff2", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "font/woff2")
		serveCacheable(writer, request, webassets.Manrope)
	})
	mux.HandleFunc("GET /static/last-light.css", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/css; charset=utf-8")
		serveCacheable(writer, request, webassets.LastLightCSS)
	})
	mux.HandleFunc("GET /static/appearance.js", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		serveCacheable(writer, request, webassets.Appearance)
	})
	mux.HandleFunc("GET /static/passkeys.js", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		serveCacheable(writer, request, webassets.Passkeys)
	})
	mux.HandleFunc("GET /manifest.webmanifest", serveFile("web/manifest.webmanifest", "application/manifest+json"))
	mux.HandleFunc("GET /service-worker.js", serveFile("web/service-worker.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /favicon.svg", serveFile("web/static/icon.svg", "image/svg+xml"))
	registerAuthentication(mux, config, authentication)
	newPasskeyAuth(config.PublicURL, authentication, config.SecureCookies).register(mux)
	registerAPI(mux, board, prober, program)
	mcpHandler := NewMCPHandler(board, prober)
	mux.Handle("/mcp", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		identity, found := currentIdentity(request)
		if !found || !identity.ViaBearer || identity.Purpose != "mcp" {
			apiError(writer, errors.New("MCP requires a bearer token"), http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(writer, request)
	}))
	registerWeb(mux, authentication)
	withIdentity := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if identity, found := authenticateRequest(authentication, request); found {
			request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity))
		}
		mux.ServeHTTP(writer, request)
	})
	return http.NewCrossOriginProtection().Handler(securityHeaders(protectHost(config.TrustedHosts, withIdentity)))
}

func authenticateRequest(authentication *auth.Manager, request *http.Request) (auth.Identity, bool) {
	values := request.Header.Values("Authorization")
	if len(values) > 0 {
		parts := strings.Fields(values[0])
		if len(values) != 1 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return auth.Identity{}, false
		}
	}
	return authentication.Authenticate(request.Context(), request)
}

func currentIdentity(request *http.Request) (auth.Identity, bool) {
	identity, found := request.Context().Value(identityKey{}).(auth.Identity)
	return identity, found
}

func requireOwner(writer http.ResponseWriter, request *http.Request) (auth.Identity, bool) {
	identity, found := currentIdentity(request)
	if !found || identity.Purpose == "mcp" {
		apiError(writer, errors.New("authentication required"), http.StatusUnauthorized)
		return auth.Identity{}, false
	}
	return identity, true
}

func requireCSRF(writer http.ResponseWriter, request *http.Request, identity auth.Identity) bool {
	if identity.ViaBearer {
		return true
	}
	provided := request.Header.Get("X-Kinosail-CSRF")
	if provided == "" || provided != identity.CSRF {
		apiError(writer, errors.New("request verification failed"), http.StatusForbidden)
		return false
	}
	return true
}

func serveAsset(prefix string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := filepath.Base(request.PathValue("name"))
		if name != request.PathValue("name") || strings.HasPrefix(name, ".") {
			http.NotFound(writer, request)
			return
		}
		path := prefix + name
		data, err := webAssets.ReadFile(path)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		if path == "web/static/dashboard.css" {
			data = dashboardCSS
		}
		contentType := mime.TypeByExtension(filepath.Ext(name))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		writer.Header().Set("Content-Type", contentType)
		serveCacheable(writer, request, data)
	}
}

func serveFile(path, contentType string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		data, err := webAssets.ReadFile(path)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", contentType)
		serveCacheable(writer, request, data)
	}
}

func serveCacheable(writer http.ResponseWriter, request *http.Request, data []byte) {
	etag := fmt.Sprintf("\"%x\"", sha256.Sum256(data))
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("ETag", etag)
	if request.Header.Get("If-None-Match") == etag {
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = writer.Write(data)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'; object-src 'none'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(writer, request)
	})
}
