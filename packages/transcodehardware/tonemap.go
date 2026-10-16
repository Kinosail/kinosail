package transcodehardware

import (
	"context"
	"slices"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func (state *verification) verifyToneMapping(ctx context.Context, options ProbeOptions, backend *Backend, baseline Operation, settings transcodepolicy.Settings, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, hdr := range []string{"hdr10", "hlg"} {
		for _, cpuFrames := range []bool{false, true} {
			state.verifyToneMapInput(ctx, options, backend, baseline, settings, hdr, cpuFrames, check)
		}
	}
}

func (state *verification) verifyToneMapInput(ctx context.Context, options ProbeOptions, backend *Backend, baseline Operation, settings transcodepolicy.Settings, hdr string, cpuFrames bool, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, method := range transcodepolicy.ToneMapMethods(backend.ID, baseline.Device) {
		if hdr == "hlg" && (method == "qsv" || method == "vaapi") {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		settings.ToneMap, settings.ToneMapInput, settings.HardwareToneMap = true, hdr, method
		settings.HardwareDecode = false
		settings.SoftwareFilters, settings.Deinterlace = cpuFrames, cpuFrames
		if !cpuFrames && transcodepolicy.ToneMapSoftwareFrames(settings) {
			continue
		}
		probeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		result := check(probeCtx, options.FFmpeg, settings, backend.Name)
		stop()
		if !verifiedToneMapResult(result, method, hdr) {
			continue
		}
		state.record(backend.ID, Operation{identity: operationIdentity(options.FFmpeg, baseline.Device), Codec: baseline.Codec, Device: baseline.Device, HardwareToneMap: method, ToneMapInput: hdr, ToneMapSoftwareFrames: cpuFrames, Status: "passed"})
		if !cpuFrames && method == backend.ID && slices.Contains(state.deviceDecodeInputs(backend.ID, baseline.Device), "hevc10") {
			state.verifyHardwareDecode(ctx, options, settings, backend, "hevc10", check)
		}
		break
	}
}

func verifiedToneMapResult(result transcodepolicy.CheckResult, method, hdr string) bool {
	return result.Status == "passed" && result.HardwareToneMap == method && result.ToneMapInput == hdr
}

func (capabilities Capabilities) toneMapSettings(options transcodepolicy.Settings) transcodepolicy.Settings {
	options.HardwareToneMap = ""
	if !options.ToneMap {
		return options
	}
	if options.DisableHardwareToneMap || options.OutputHDR != "" || capabilities.verification == nil {
		options.HardwareDecode = false
		return options
	}
	for _, operation := range capabilities.verification.operationsFor(options.Accelerator, options.Codec) {
		if operation.Device != options.Device || operation.HardwareToneMap == "" || operation.ToneMapInput != options.ToneMapInput {
			continue
		}
		candidate := options
		candidate.HardwareToneMap = operation.HardwareToneMap
		if operation.ToneMapSoftwareFrames != transcodepolicy.ToneMapSoftwareFrames(candidate) {
			continue
		}
		candidate.HardwareDecode = options.HardwareDecode && operation.decodes(options.DecodeInput)
		return candidate
	}
	options.HardwareDecode = false
	return options
}

func (capabilities Capabilities) RecordProcessingFailure(options transcodepolicy.Settings) {
	if options.HardwareToneMap != "" && capabilities.verification != nil {
		if options.HardwareDecode {
			capabilities.RecordDecodeFailure(options)
			return
		}
		state := capabilities.verification
		state.mu.Lock()
		defer state.mu.Unlock()
		for index := range state.operations[options.Accelerator] {
			operation := &state.operations[options.Accelerator][index]
			if operation.Device == options.Device && operation.Codec == options.Codec && operation.HardwareToneMap == options.HardwareToneMap && operation.ToneMapInput == options.ToneMapInput && operation.ToneMapSoftwareFrames == transcodepolicy.ToneMapSoftwareFrames(options) {
				operation.Status = "failed"
			}
		}
		return
	}
	if options.HardwareDecode {
		capabilities.RecordDecodeFailure(options)
	} else {
		capabilities.RecordFailure(options)
	}
}

func (state *verification) verifyToneMaps(parent context.Context, options ProbeOptions, backends []Backend, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	eachBackend(backends, func(backend *Backend) {
		for _, operation := range state.snapshot(backend.ID) {
			if operation.Status != "passed" || operation.OutputHDR != "" || operation.HardwareToneMap != "" {
				continue
			}
			settings := transcodepolicy.Settings{Codec: operation.Codec, Accelerator: backend.ID, Encoder: backend.encoders[operation.Codec], Device: operation.Device, Preset: "veryfast", CRF: "22"}
			state.verifyToneMapping(ctx, options, backend, operation, settings, check)
		}
		backend.Operations = state.snapshot(backend.ID)
	})
}
