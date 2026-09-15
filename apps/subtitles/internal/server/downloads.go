package server

import (
	"context"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"

	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/workload"
)

func newDownloadManager(ctx context.Context, cache, ffmpeg string, settings *settingsStore, workloads *workload.Governor, probes ...*mediaProbe) *downloadManager {
	config := downloadConfiguration(ctx, cache, ffmpeg, settings, workloads)
	if len(probes) > 0 && probes[0] != nil {
		config.Inspect = func(ctx context.Context, item library.Item) playback.MediaFacts {
			return mediaFactsFor(item, probes[0].inspect(ctx, item))
		}
	}
	config.AcquireEncoding = func(ctx context.Context, options transcodepolicy.Settings) (func(), error) {
		return workloads.AcquireEncoding(ctx, workload.Background, 1, transcodehardware.DeviceKey(options))
	}
	return &downloadManager{downloads.New(config)}
}
