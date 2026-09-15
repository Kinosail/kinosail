package jellyfincompat

import (
	"net/http"
	"time"
)

// WriteJSON writes one Jellyfin-compatible response.
type WriteJSON func(http.ResponseWriter, any)

// ItemHandlers contains app-owned Jellyfin item HTTP adapters.
type ItemHandlers struct {
	Views, Items, Latest, Persons, Item, Seasons, Episodes, NextUp, Resume http.HandlerFunc
}

// RegisterItems registers the shared Jellyfin item route contract.
func RegisterItems(mux *http.ServeMux, handlers ItemHandlers) {
	mux.HandleFunc("GET /UserViews", handlers.Views)
	mux.HandleFunc("GET /Users/{user}/Views", handlers.Views)
	mux.HandleFunc("GET /Items", handlers.Items)
	mux.HandleFunc("GET /Users/{user}/Items", handlers.Items)
	mux.HandleFunc("GET /Items/Latest", handlers.Latest)
	mux.HandleFunc("GET /Users/{user}/Items/Latest", handlers.Latest)
	mux.HandleFunc("GET /Persons", handlers.Persons)
	mux.HandleFunc("GET /Items/{id}", handlers.Item)
	mux.HandleFunc("GET /Users/{user}/Items/{id}", handlers.Item)
	mux.HandleFunc("GET /Shows/{id}/Seasons", handlers.Seasons)
	mux.HandleFunc("GET /Shows/{id}/Episodes", handlers.Episodes)
	mux.HandleFunc("GET /Shows/NextUp", handlers.NextUp)
	mux.HandleFunc("GET /UserItems/Resume", handlers.Resume)
}

// Users writes visible profile data through the app-owned projection.
func Users[T any](writer http.ResponseWriter, profiles []T, visible func(T) (map[string]any, bool), write WriteJSON) {
	users := make([]map[string]any, 0, len(profiles))
	for _, profile := range profiles {
		if user, include := visible(profile); include {
			users = append(users, user)
		}
	}
	write(writer, users)
}

// CreateAuthKey validates one app and records its short-lived key handoff.
func CreateAuthKey(writer http.ResponseWriter, request *http.Request, profile string, create func(string) (string, error), keyID func(string) string, grants *Grants, now func() time.Time) {
	app, ok := App(request.URL.Query())
	if !ok {
		http.Error(writer, "unsupported application", http.StatusBadRequest)
		return
	}
	secret, err := create(app)
	if err != nil {
		http.Error(writer, "could not create API key", http.StatusInternalServerError)
		return
	}
	grants.Store(profile, app, secret, keyID(secret), now().Add(5*time.Minute))
	writer.WriteHeader(http.StatusNoContent)
}

// AuthKeys writes active key handoffs for one profile.
func AuthKeys(writer http.ResponseWriter, profile string, grants *Grants, now time.Time, write WriteJSON) {
	items := grants.Items(profile, now)
	write(writer, map[string]any{"Items": items, "TotalRecordCount": len(items)})
}
