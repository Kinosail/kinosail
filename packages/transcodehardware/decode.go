package transcodehardware

import (
	"context"
	"slices"
	"time"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func (operation Operation) decodes(input string) bool {
	return input != "" && slices.Contains(operation.DecodeInputs, input)
}

func (operation Operation) matches(other Operation) bool {
	return operation.Codec == other.Codec && operation.Device == other.Device && operation.OutputHDR == other.OutputHDR && operation.HardwareToneMap == other.HardwareToneMap && operation.ToneMapInput == other.ToneMapInput && operation.ToneMapSoftwareFrames == other.ToneMapSoftwareFrames
}

func (operation Operation) baseline() bool {
	return operation.OutputHDR == "" && operation.HardwareToneMap == ""
}

func mergeDecodeInputs(previous, current []string) []string {
	merged := append([]string(nil), previous...)
	for _, input := range current {
		if !slices.Contains(merged, input) {
			merged = append(merged, input)
		}
	}
	return merged
}

func decodeTarget(options transcodepolicy.Settings) Operation {
	return Operation{Codec: options.Codec, Device: options.Device, OutputHDR: options.OutputHDR, HardwareToneMap: options.HardwareToneMap, ToneMapInput: options.ToneMapInput, ToneMapSoftwareFrames: options.HardwareToneMap != "" && transcodepolicy.ToneMapSoftwareFrames(options)}
}

func (target Operation) covers(operation Operation) bool {
	if target.baseline() {
		return operation.baseline() && operation.Device == target.Device
	}
	return operation.matches(target)
}

func (state *verification) deviceDecodeInputs(backend, device string) []string {
	state.mu.RLock()
	defer state.mu.RUnlock()
	var result []string
	for _, operation := range state.operations[backend] {
		if operation.Device == device && operation.Status == "passed" && operation.baseline() {
			result = mergeDecodeInputs(result, operation.DecodeInputs)
		}
	}
	return result
}

func (state *verification) addDecodeInput(backend string, target Operation, input string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	for index := range state.operations[backend] {
		operation := &state.operations[backend][index]
		if operation.Status == "passed" && target.covers(*operation) {
			operation.DecodeInputs = mergeDecodeInputs(operation.DecodeInputs, []string{input})
		}
	}
}

func (state *verification) verifyHardwareDecode(ctx context.Context, options ProbeOptions, settings transcodepolicy.Settings, backend *Backend, input string, check func(context.Context, string, transcodepolicy.Settings, string) transcodepolicy.CheckResult) {
	if !transcodepolicy.HardwareDecodeInput(backend.ID) || ctx.Err() != nil {
		return
	}
	settings.HardwareDecode, settings.DecodeInput = true, input
	probeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	if result := check(probeCtx, options.FFmpeg, settings, backend.Name); verifiedDecodeResult(result, input) {
		state.addDecodeInput(backend.ID, decodeTarget(settings), input)
	}
}

func verifiedDecodeResult(result transcodepolicy.CheckResult, input string) bool {
	return result.Status == "passed" && result.HardwareDecode && result.DecodeInput == input
}

func (capabilities Capabilities) RecordDecodeFailure(options transcodepolicy.Settings) {
	if capabilities.verification == nil || options.DecodeInput == "" {
		return
	}
	state := capabilities.verification
	state.mu.Lock()
	defer state.mu.Unlock()
	target := decodeTarget(options)
	for index := range state.operations[options.Accelerator] {
		operation := &state.operations[options.Accelerator][index]
		if target.covers(*operation) {
			operation.DecodeInputs = slices.DeleteFunc(append([]string(nil), operation.DecodeInputs...), func(input string) bool { return input == options.DecodeInput })
		}
	}
}
