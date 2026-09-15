package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestLibraryMastheadEyebrowStaysAboveContrastWash(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{})
	styles := httptest.NewRecorder()
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	if !strings.Contains(styles.Body.String(), `.library-masthead .eyebrow{position:relative;z-index:1}`) {
		t.Fatalf("masthead eyebrow must remain above its contrast wash: %q", styles.Body.String())
	}
}
