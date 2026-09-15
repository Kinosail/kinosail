package main

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestHealthcheckUsesConfiguredAuthHost(t *testing.T) {
	commandtest.HealthcheckConfiguredAuthHost(t, unusedAddress, startTestServer, stopTestServer, assertHealthcheck, testServerStartTimeout)
}
