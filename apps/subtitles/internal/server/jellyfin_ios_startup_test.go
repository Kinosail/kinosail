package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestJellyfinIOSCanValidateServer(t *testing.T) {
	t.Parallel()
	handler := newJellyfinServer(t, server.Config{RequireAuth: true})
	response := jellyfinCall(t, handler, http.MethodGet, "/system/info/public", "", "")
	var info struct {
		ID      string `json:"Id"`
		Product string `json:"ProductName"`
	}
	decodeJellyfin(t, response, &info)
	if response.Code != http.StatusOK || len(info.ID) != 32 || info.Product != "Jellyfin Server" {
		t.Fatalf("iOS public info = %d %q", response.Code, response.Body.String())
	}
}
