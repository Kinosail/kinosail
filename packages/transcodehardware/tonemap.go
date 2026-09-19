package transcodehardware

import (
	"context"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func (state *verification) verifyToneMapping(ctx context.Context, options ProbeOptions, backend *Backend, baseline Operation, settings transcodepolicy.Settings, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, hdr := range []string{"hdr10", "hlg"} {
		for _, cpuFrames := range []bool{false, true} {
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
				if result.Status != "passed" || result.HardwareToneMap != method || result.ToneMapInput != hdr {
					continue
				}
				state.operations[backend.ID] = append(state.operations[backend.ID], Operation{identity: operationIdentity(options.FFmpeg, baseline.Device), Codec: baseline.Codec, Device: baseline.Device, HardwareToneMap: method, ToneMapInput: hdr, ToneMapSoftwareFrames: cpuFrames, Status: "passed"})
				break
			}
		}
	}
}

func (capabilities Capabilities) toneMapSettings(options transcodepolicy.Settings) transcodepolicy.Settings {
	options.HardwareToneMap = ""
	if !options.ToneMap || options.DisableHardwareToneMap || options.OutputHDR != "" || capabilities.verification == nil {
		return options
	}
	for _, operation := range capabilities.verification.operationsFor(options.Accelerator, options.Codec) {
		if operation.Device == options.Device && operation.HardwareToneMap != "" && operation.ToneMapInput == options.ToneMapInput {
			options.HardwareToneMap = operation.HardwareToneMap
			if operation.ToneMapSoftwareFrames != transcodepolicy.ToneMapSoftwareFrames(options) {
				options.HardwareToneMap = ""
				continue
			}
			options.HardwareDecode = false
			break
		}
	}
	return options
}

// RecordProcessingFailure quarantines only the failed stage. A tone mapper
// failure must not discard independently verified encoding on the same GPU.
func (capabilities Capabilities) RecordProcessingFailure(options transcodepolicy.Settings) {
	if options.HardwareToneMap != "" && capabilities.verification != nil {
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

// Optional HDR checks have their own bounded budget after codec/decode/output
// checks, so a slow tone mapper cannot remove an otherwise usable codec.
func (state *verification) verifyToneMaps(parent context.Context, options ProbeOptions, backends []Backend, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	for index := range backends {
		backend := &backends[index]
		for _, operation := range append([]Operation(nil), state.operations[backend.ID]...) {
			if operation.Status != "passed" || operation.OutputHDR != "" || operation.HardwareToneMap != "" {
				continue
			}
			settings := transcodepolicy.Settings{Codec: operation.Codec, Accelerator: backend.ID, Encoder: backend.encoders[operation.Codec], Device: operation.Device, Preset: "veryfast", CRF: "22"}
			state.verifyToneMapping(ctx, options, backend, operation, settings, check)
		}
		backend.Operations = append([]Operation(nil), state.operations[backend.ID]...)
	}
}
