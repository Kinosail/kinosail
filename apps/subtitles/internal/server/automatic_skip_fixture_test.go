package server_test

import (
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var automaticSkipFixture = servertest.NewAutomaticSkipFixture(
	func(fixture servertest.AutomaticSkipConfig) server.Config {
		return server.Config{Lifecycle: fixture.Lifecycle, MediaDir: fixture.MediaDir, DataDir: fixture.DataDir, CacheDir: fixture.CacheDir, FFprobe: fixture.FFprobe, FFmpeg: fixture.FFmpeg, RequireAuth: true}
	}, server.New, newJellyfinServer, testTOTP, false, 8)
var jellyfinRequest = servertest.AutomaticSkipJellyfinRequest
