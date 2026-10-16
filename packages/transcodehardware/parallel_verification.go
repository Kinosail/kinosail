package transcodehardware

import (
	"context"
	"sync"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

const parallelBackendChecks = 4

func eachBackend(backends []Backend, run func(*Backend)) {
	var group sync.WaitGroup
	slots := make(chan struct{}, parallelBackendChecks)
	for index := range backends {
		backend := &backends[index]
		group.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			run(backend)
		})
	}
	group.Wait()
}

func (state *verification) snapshot(backend string) []Operation {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return append([]Operation(nil), state.operations[backend]...)
}

func (state *verification) record(backend string, operation Operation) {
	state.mu.Lock()
	state.operations[backend] = append(state.operations[backend], operation)
	state.mu.Unlock()
}

func (state *verification) verifyBaselines(ctx context.Context, options ProbeOptions, backends []Backend, devices []string, eligible map[string]bool, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, codec := range []string{"h264", "hevc", "av1", "vp9"} {
		eachBackend(backends, func(backend *Backend) {
			if eligible[backend.ID] {
				state.verifyCodec(ctx, options, backend, codec, devices, check)
			}
		})
		if codec == "h264" {
			eachBackend(backends, func(backend *Backend) { state.verifyBaselineDecode(ctx, options, backend, codec, check) })
		}
	}
}
