package transcodehardware

import (
	"context"
	"slices"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func (state *verification) verifyCodec(ctx context.Context, options ProbeOptions, backend *Backend, codec string, devices []string, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, device := range backendDevices(*backend, devices) {
		encoder := backend.encoders[codec]
		if encoder == "" || ctx.Err() != nil {
			continue
		}
		settings := transcodepolicy.Settings{Name: "automatic", Codec: codec, Accelerator: backend.ID, Encoder: encoder, Device: device, Preset: "veryfast", CRF: "22"}
		probeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		result := check(probeCtx, options.FFmpeg, settings, backend.Name)
		stop()
		operation := Operation{identity: operationIdentity(options.FFmpeg, device), Codec: codec, Device: device, Status: result.Status}
		if result.Status == "passed" {
			operation.DecodeInputs = state.deviceDecodeInputs(backend.ID, device)
			backend.Usable = true
		}
		state.record(backend.ID, operation)
	}
}

func (state *verification) verifyBaselineDecode(ctx context.Context, options ProbeOptions, backend *Backend, codec string, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, operation := range state.snapshot(backend.ID) {
		if operation.Status != "passed" || operation.HardwareToneMap != "" {
			continue
		}
		settings := transcodepolicy.Settings{Name: "automatic", Codec: codec, Accelerator: backend.ID, Encoder: backend.encoders[codec], Device: operation.Device, Preset: "veryfast", CRF: "22"}
		state.verifyHardwareDecode(ctx, options, settings, backend, "h264", check)
	}
}

func (state *verification) verifyOptionalFormats(ctx context.Context, options ProbeOptions, backend *Backend, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	baseline := state.snapshot(backend.ID)
	inputs := transcodepolicy.DecodeInputs()[1:]
	state.verifyDecodeInputs(ctx, options, backend, baseline, inputs[:2], check)
	for _, operation := range baseline {
		if operation.Status != "passed" || !operation.baseline() || operation.Codec != "hevc" {
			continue
		}
		settings := transcodepolicy.Settings{Name: "automatic", Codec: operation.Codec, Accelerator: backend.ID, Encoder: backend.encoders[operation.Codec], Device: operation.Device, Preset: "veryfast", CRF: "22"}
		state.verifyHDR(ctx, options, backend, operation, settings, check)
	}
	state.verifyDecodeInputs(ctx, options, backend, baseline, inputs[2:], check)
}

func (state *verification) verifyDecodeInputs(ctx context.Context, options ProbeOptions, backend *Backend, baseline []Operation, inputs []string, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, operation := range baseline {
		if operation.Status != "passed" || !operation.baseline() || operation.Codec != "h264" {
			continue
		}
		settings := transcodepolicy.Settings{Name: "automatic", Codec: operation.Codec, Accelerator: backend.ID, Encoder: backend.encoders[operation.Codec], Device: operation.Device, Preset: "veryfast", CRF: "22"}
		for _, input := range inputs {
			state.verifyHardwareDecode(ctx, options, settings, backend, input, check)
		}
	}
}

func (state *verification) verifyHDR(ctx context.Context, options ProbeOptions, backend *Backend, operation Operation, settings transcodepolicy.Settings, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, hdr := range []string{"hdr10", "hlg"} {
		if ctx.Err() != nil {
			break
		}
		settings.OutputHDR = hdr
		probeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		result := check(probeCtx, options.FFmpeg, settings, backend.Name)
		stop()
		state.record(backend.ID, Operation{identity: operationIdentity(options.FFmpeg, operation.Device), Codec: operation.Codec, Device: operation.Device, OutputHDR: hdr, Status: result.Status})
		if result.Status == "passed" && slices.Contains(state.deviceDecodeInputs(backend.ID, operation.Device), "hevc10") {
			state.verifyHardwareDecode(ctx, options, settings, backend, "hevc10", check)
		}
	}
}
