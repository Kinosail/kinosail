package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestDLNALabelExplainsUnconfiguredAndConfiguredStates(t *testing.T) {
	settings := newSettingsStore("", "", "", nil)
	servertest.AssertDLNALabelExplainsUnconfiguredAndConfiguredStates(t, func() string { return dlnaLabel(settings) }, func(url, token string) { settings.dlnaURL = url; settings.value.DLNAToken = token })
}

func TestPlayerFormattingUsesStableHumanReadableBoundaries(t *testing.T) {
	servertest.AssertPlayerFormattingUsesStableHumanReadableBoundaries(t, byteSize, oneOf)
}
