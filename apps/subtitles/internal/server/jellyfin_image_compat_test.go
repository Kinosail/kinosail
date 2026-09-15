package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestJellyfinCompatibilityAllowsOnlyLocalAnonymousArtwork(t *testing.T) {
	t.Parallel()
	handler, token, _ := jellyfinTestServer(t)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items?IncludeItemTypes=Movie", "", token)
	var library jellyfinItems
	decodeJellyfin(t, items, &library)
	if len(library.Items) == 0 {
		t.Fatalf("movies = %d %q", items.Code, items.Body.String())
	}
	path := "/Items/" + library.Items[0].ID + "/Images/Primary"
	if response := jellyfinCall(t, handler, http.MethodGet, path, "", ""); response.Code != http.StatusOK {
		t.Fatalf("local anonymous artwork = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, server.Remote(handler), http.MethodGet, path, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("public anonymous artwork = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodGet, path, "", "revoked-or-invalid"); response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid artwork credential = %d %q", response.Code, response.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodGet, "/Items", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("local anonymous library = %d %q", response.Code, response.Body.String())
	}
}
