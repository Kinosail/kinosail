package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPlayerFormattingUsesStableHumanReadableBoundaries(t *testing.T) {
	servertest.AssertPlayerFormattingUsesStableHumanReadableBoundaries(t, byteSize, oneOf)
}
