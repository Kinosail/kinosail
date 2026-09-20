package server_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHLSSchedulerContract(t *testing.T) {
	servertest.HLSScheduler(t, func(config servertest.HLSSchedulerConfig) http.Handler {
		probe := filepath.Join(t.TempDir(), "ffprobe")
		writeExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"hevc","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac","index":1}],"format":{"duration":"120"}}'
`)
		return server.New(server.Config{ProbeHardware: config.ProbeHardware, HardwareDevices: config.HardwareDevices, FFprobe: probe, MediaDir: config.MediaDir, DataDir: config.DataDir, CacheDir: config.CacheDir, FFmpeg: config.FFmpeg, HardwareOS: config.HardwareOS, HardwareArch: config.HardwareArch})
	}, fakePlayableHLS())
}

var firstWebItem = servertest.WebItemFactory(server.New)
