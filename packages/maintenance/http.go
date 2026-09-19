package maintenance

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
)

// StatusHandler serves one current maintenance projection as private JSON.
func (manager *Manager) StatusHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		apihttp.WriteJSON(writer, manager.Status(), http.StatusOK)
	}
}
