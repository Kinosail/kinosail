package server_test

import (
	"net/http"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func settingsSearchHandler(dataDir string) http.Handler {
	return server.New(server.Config{DataDir: dataDir})
}
