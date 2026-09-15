package servertest

import (
	"bytes"
	"errors"
	"testing"
)

// HLSDiagnosticPreservesCauseAndKeepsOnlyRecentOutput verifies the bounded diagnostic contract.
func HLSDiagnosticPreservesCauseAndKeepsOnlyRecentOutput(t *testing.T, cause, failure error, output interface {
	Write([]byte) (int, error)
	Bytes() []byte
}, limit int,
) {
	t.Helper()
	if !errors.Is(failure, cause) {
		t.Fatal("diagnostic error did not preserve its cause")
	}
	input := bytes.Repeat([]byte("x"), limit+1)
	if written, err := output.Write(input); err != nil || written != len(input) || len(output.Bytes()) != limit || output.Bytes()[0] != 'x' {
		t.Fatalf("diagnostic output = written %d, error %v, length %d", written, err, len(output.Bytes()))
	}
}
