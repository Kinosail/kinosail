package transcodehardware

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

type checkFunc = func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult

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

func (state *verification) markDecode(backend, device, codec, input string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	for index := range state.operations[backend] {
		operation := &state.operations[backend][index]
		if operation.Device != device || operation.Codec != codec || operation.OutputHDR != "" || operation.HardwareToneMap != "" {
			continue
		}
		switch input {
		case "hevc":
			operation.DecodeHEVC = true
		case "hevc10":
			operation.DecodeHEVC10 = true
		case "av1":
			operation.DecodeAV1 = true
		default:
			operation.DecodeH264 = true
		}
	}
}

func (state *verification) verifyBaselines(ctx context.Context, options ProbeOptions, backends []Backend, devices []string, eligible map[string]bool, check checkFunc) {
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

func hardwareDecodeBackend(id string) bool {
	return slices.Contains([]string{"qsv", "vaapi", "cuda", "rkmpp", "videotoolbox"}, id)
}

func verifyHardwareDecode(ctx context.Context, options ProbeOptions, settings transcodepolicy.Settings, backend *Backend, state *verification, check checkFunc) bool {
	if !hardwareDecodeBackend(backend.ID) || ctx.Err() != nil {
		return false
	}
	settings.HardwareDecode = true
	probeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	result := check(probeCtx, options.FFmpeg, settings, backend.Name)
	if result.Status != "passed" {
		return false
	}
	state.markDecode(backend.ID, settings.Device, settings.Codec, settings.DecodeInput)
	return true
}

func (state *verification) verifyDecodeInputs(ctx context.Context, options ProbeOptions, backend *Backend, operation Operation, settings transcodepolicy.Settings, check checkFunc) {
	if operation.Codec != "h264" || !operation.DecodeH264 {
		return
	}
	for _, input := range []string{"hevc", "hevc10", "av1"} {
		settings.DecodeInput = input
		verifyHardwareDecode(ctx, options, settings, backend, state, check)
	}
}
