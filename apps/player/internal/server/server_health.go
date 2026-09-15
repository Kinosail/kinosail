package server

import "net/http"

func health(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write([]byte("{\"status\":\"ok\"}\n"))
}
