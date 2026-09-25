package server_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var downloadFixture = servertest.DownloadFixture{
	DownloadsScript: `/static/downloads.js?v=13`,
	APIServer:       apiServer,
	FirstItemID:     firstAPIItemID,
	SignIn:          signInTestProfile,
	APICall:         apiCall,
	CookieRequest:   requestWithCookie,
	NewHandler: func(media, data, cache string) http.Handler {
		return server.New(server.Config{MediaDir: media, DataDir: data, CacheDir: cache, RequireAuth: true})
	},
	NewTranscodeHandler: func(media, data, cache, ffmpeg string) http.Handler {
		return server.New(server.Config{MediaDir: media, DataDir: data, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: filepath.Join(filepath.Dir(ffmpeg), "ffprobe"), RequireAuth: true})
	},
}

func TestDownloadsBoundaryContract(t *testing.T) {
	servertest.DownloadsBoundaryContract(t, downloadFixture)
}
