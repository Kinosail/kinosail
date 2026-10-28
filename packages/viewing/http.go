package viewing

import (
	"errors"
	"net/http"
)

var (
	ErrSyncNotFound = errors.New("viewing activity sync was not found")
	ErrSyncRunning  = errors.New("viewing activity sync is already running")
	ErrSyncCapacity = errors.New("viewing activity sync capacity is in use")
)

func SyncRunStatus(err error) int {
	switch {
	case errors.Is(err, ErrSyncNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrSyncRunning), errors.Is(err, ErrSyncCapacity):
		return http.StatusConflict
	}
	return http.StatusBadGateway
}

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
