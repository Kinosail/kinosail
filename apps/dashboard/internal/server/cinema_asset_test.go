package server

import (
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCinemaBackgroundServesLandscapeAsset(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	newTestApplication(t, Config{}).handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/cinema-backdrop.jpg?v=2", nil))
	config, err := jpeg.DecodeConfig(response.Body)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/jpeg" || err != nil || config.Width != 1920 || config.Height != 1080 {
		t.Fatalf("cinema asset: status=%d dimensions=%dx%d error=%v", response.Code, config.Width, config.Height, err)
	}
}
