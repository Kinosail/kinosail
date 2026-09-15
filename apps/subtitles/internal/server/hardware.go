package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/transcodehardware"
)

type (
	hardwareBackend      = transcodehardware.Backend
	hardwareCapabilities = transcodehardware.Capabilities
)

func probeHardware(ctx context.Context, ffmpeg string, devices []string, enabled bool, goos, goarch string) hardwareCapabilities {
	return transcodehardware.Probe(ctx, transcodehardware.ProbeOptions{
		Application: "Kinosail Subtitles",
		FFmpeg:      ffmpeg,
		Devices:     devices,
		Enabled:     enabled,
		GOOS:        goos,
		GOARCH:      goarch,
	})
}
