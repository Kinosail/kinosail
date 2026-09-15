package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/workload"
)

func downloadConfiguration(ctx context.Context, cache, ffmpeg string, settings *settingsStore, workloads *workload.Governor) downloads.Config {
	return downloads.Config{Context: ctx, Cache: cache, FFmpeg: ffmpeg, Transcoding: downloadTranscoding(settings), Acquire: func(ctx context.Context) (func(), error) { return workloads.Acquire(ctx, workload.Background) }, Persist: saveJSON}
}
