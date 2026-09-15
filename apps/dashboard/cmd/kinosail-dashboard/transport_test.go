package main

import (
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestHTTPServerUsesSharedPolicyAndPreservesDashboardLimits(t *testing.T) {
	commandtest.HTTPServerPolicy(t, newHTTPServer, commandtest.HTTPServerLimits{
		Address: "127.0.0.1:38400", Read: 20 * time.Second, Write: 35 * time.Second,
		Idle: 2 * time.Minute, HeaderBytes: 64 << 10,
	})
}
