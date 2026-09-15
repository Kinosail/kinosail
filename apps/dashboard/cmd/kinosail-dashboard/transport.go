package main

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/servertransport"
)

func newHTTPServer(address string, handler http.Handler) *http.Server {
	server := servertransport.NewServer(address, handler)
	server.ReadTimeout = 20 * time.Second
	server.WriteTimeout = 35 * time.Second
	return server
}
