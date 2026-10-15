package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

var securityFuzz = servertest.SecurityFuzz{AllowedOutboundIP: allowedOutboundIP, UnsafeCrossOrigin: unsafeCrossOrigin}

func FuzzRemoteBoundaryParsers(f *testing.F) {
	securityFuzz.RemoteBoundaryParsers(f)
}

func FuzzExactBrowserOrigin(f *testing.F) {
	securityFuzz.ExactBrowserOrigin(f)
}
