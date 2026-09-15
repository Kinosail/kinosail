package server

import (
	"net/http"
)

func unavailableApplication(config Config, message string) http.Handler {
	if config.InternetAccess != nil {
		_ = config.InternetAccess.Kill()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, message, http.StatusServiceUnavailable)
	})
}
