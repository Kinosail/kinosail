package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHLSSchedulerContract(t *testing.T) {
	servertest.HLSScheduler(t, func(config servertest.HLSSchedulerConfig) http.Handler {
		return server.New(server.Config{MediaDir: config.MediaDir, DataDir: config.DataDir, CacheDir: config.CacheDir, FFmpeg: config.FFmpeg, HardwareOS: config.HardwareOS, HardwareArch: config.HardwareArch})
	}, fakePlayableHLS())
}

var firstWebItem = servertest.WebItemFactory(server.New)
