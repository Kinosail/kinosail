package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLocalizedPageRender(b *testing.B) {
	media := b.TempDir()
	for index := range 50 {
		if err := os.WriteFile(filepath.Join(media, fmt.Sprintf("Signal Garden %02d (2020).mp4", index)), []byte("video"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	handler := New(Config{Lifecycle: b.Context(), MediaDir: media, DataDir: b.TempDir()})
	for _, target := range []string{"/", "/login", "/settings/backups"} {
		b.Run(target, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "https://kinosail.test"+target, nil)
				request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
				request.Header.Set("Accept-Language", "es")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					b.Fatalf("%s = %d", target, response.Code)
				}
			}
		})
	}
}
