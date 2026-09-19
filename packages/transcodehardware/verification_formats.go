package transcodehardware

import (
	"context"
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
		state.operations[backend.ID] = append(state.operations[backend.ID], Operation{identity: operationIdentity(options.FFmpeg, device), Codec: codec, Device: device, Status: result.Status})
		if result.Status == "passed" {
			backend.Usable = true
		}
	}
}

func (state *verification) verifyBaselineDecode(ctx context.Context, options ProbeOptions, backend *Backend, codec string, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	for _, operation := range state.operations[backend.ID] {
		if operation.Status != "passed" || operation.HardwareToneMap != "" {
			continue
		}
		settings := transcodepolicy.Settings{Name: "automatic", Codec: codec, Accelerator: backend.ID, Encoder: backend.encoders[codec], Device: operation.Device, Preset: "veryfast", CRF: "22"}
		verifyHardwareDecode(ctx, options, settings, backend, state, check)
	}
}

func (state *verification) verifyOptionalFormats(ctx context.Context, options ProbeOptions, backend *Backend, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	baseline := append([]Operation(nil), state.operations[backend.ID]...)
	for _, operation := range baseline {
		if operation.Status != "passed" || operation.HardwareToneMap != "" {
			continue
		}
		settings := transcodepolicy.Settings{Name: "automatic", Codec: operation.Codec, Accelerator: backend.ID, Encoder: backend.encoders[operation.Codec], Device: operation.Device, Preset: "veryfast", CRF: "22"}
		if operation.Codec != "h264" {
			verifyHardwareDecode(ctx, options, settings, backend, state, check)
		}
		if operation.Codec == "hevc" {
			state.verifyHDR(ctx, options, backend, operation, settings, check)
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
		state.operations[backend.ID] = append(state.operations[backend.ID], Operation{identity: operationIdentity(options.FFmpeg, operation.Device), Codec: operation.Codec, Device: operation.Device, OutputHDR: hdr, Status: result.Status})
	}
}
