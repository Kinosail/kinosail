package server

import (
	"context"
	"os/exec"

	"github.com/MikeO7/kinosail/packages/playback"
)

const hlsDiagnosticLimit = playback.HLSDiagnosticLimit

type (
	hlsDiagnosticError  = playback.HLSDiagnosticError
	hlsDiagnosticBuffer = playback.HLSDiagnosticBuffer
)

func newHLSDiagnosticError(cause error, detail string, private ...string) *hlsDiagnosticError {
	return playback.NewHLSDiagnosticError(cause, detail, private...)
}

func hlsDiagnostic(err error, private ...string) string {
	return playback.HLSDiagnostic(err, private...)
}

func runHLSCommand(ctx context.Context, command *exec.Cmd, private ...string) error {
	observation := hlsObservationFor(ctx)
	started := false
	err := playback.RunHLSCommandObserved(command, func() { started = true; observation.emit("process_started", "") }, private...)
	if err != nil && !started {
		observation.emit("process_start_failed", "failed")
	}
	return err
}
