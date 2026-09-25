package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestDLNALabelExplainsUnconfiguredAndConfiguredStates(t *testing.T) {
	settings := newSettingsStore("", "", "", nil)
	servertest.AssertDLNALabelExplainsUnconfiguredAndConfiguredStates(t, func() string { return dlnaLabel(settings) }, func(url, token string) { settings.dlnaURL = url; settings.value.DLNAToken = token })
}

func TestHardwareSupportAcceptsAutomaticAndKnownBackends(t *testing.T) {
	capabilities := hardwareCapabilities{Selected: "software", Backends: []hardwareBackend{{ID: "vaapi", Supported: true}}}
	servertest.AssertHardwareSupportAcceptsAutomaticAndKnownBackends(t, capabilities.Supports)
}

func TestPlayerFormattingUsesStableHumanReadableBoundaries(t *testing.T) {
	servertest.AssertPlayerFormattingUsesStableHumanReadableBoundaries(t, byteSize, oneOf)
}
