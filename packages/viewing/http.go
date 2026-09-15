package viewing

import "net/http"

// HTTPHandlers is the complete viewing import and recurring-sync transport surface.
type HTTPHandlers interface {
	APIPreview(http.ResponseWriter, *http.Request)
	APIApply(http.ResponseWriter, *http.Request)
	APIListSyncs(http.ResponseWriter, *http.Request)
	APICreateSync(http.ResponseWriter, *http.Request)
	APIRunSync(http.ResponseWriter, *http.Request)
	APIDeleteSync(http.ResponseWriter, *http.Request)
	WebPreview(http.ResponseWriter, *http.Request)
	WebApply(http.ResponseWriter, *http.Request)
	WebOnboarding(http.ResponseWriter, *http.Request)
	WebCreateSync(http.ResponseWriter, *http.Request)
	WebRunSync(http.ResponseWriter, *http.Request)
	WebDeleteSync(http.ResponseWriter, *http.Request)
}

// RegisterHTTP installs the Player-canonical viewing import route set.
func RegisterHTTP(mux *http.ServeMux, owner func(http.Handler) http.Handler, handlers HTTPHandlers) {
	handle := func(pattern string, handler http.HandlerFunc) { mux.Handle(pattern, owner(handler)) }
	handle("POST /api/v1/viewing-imports/preview", handlers.APIPreview)
	handle("POST /api/v1/viewing-imports/{id}/apply", handlers.APIApply)
	handle("GET /api/v1/viewing-syncs", handlers.APIListSyncs)
	handle("POST /api/v1/viewing-syncs", handlers.APICreateSync)
	handle("POST /api/v1/viewing-syncs/{id}/run", handlers.APIRunSync)
	handle("DELETE /api/v1/viewing-syncs/{id}", handlers.APIDeleteSync)
	handle("POST /settings/viewing-imports/preview", handlers.WebPreview)
	handle("POST /settings/viewing-imports/apply", handlers.WebApply)
	handle("GET /onboarding/migrate", handlers.WebOnboarding)
	handle("POST /onboarding/viewing-imports/preview", handlers.WebPreview)
	handle("POST /onboarding/viewing-imports/apply", handlers.WebApply)
	handle("POST /settings/viewing-syncs", handlers.WebCreateSync)
	handle("POST /settings/viewing-syncs/run", handlers.WebRunSync)
	handle("POST /settings/viewing-syncs/remove", handlers.WebDeleteSync)
}
