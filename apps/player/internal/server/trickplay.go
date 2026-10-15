package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

type trickplay struct{ register func(*http.ServeMux) }

func newTrickplay(cache, ffmpeg string, index *libraryIndex, workloads *workload.Governor) trickplay {
	acquire := func(ctx context.Context) (func(), error) { return workloads.Acquire(ctx, workload.Background) }
	return trickplay{register: playback.MustTrickplayRegistration(cache, ffmpeg, hlsPolicy(), index.VisibleItem, acquire, localizedError)}
}
