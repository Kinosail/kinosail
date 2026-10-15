package maintenance

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
)

func (manager *Manager) StatusHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		apihttp.WriteJSON(writer, manager.Status(), http.StatusOK)
	}
}
