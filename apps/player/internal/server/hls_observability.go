package server

import (
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

func runHLSCommand(command *exec.Cmd, private ...string) error {
	return playback.RunHLSCommand(command, private...)
}
