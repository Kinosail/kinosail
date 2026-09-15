package dashboard

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
)

func TestProbeErrorTranslationPreservesWrappedNetworkCauses(t *testing.T) {
	for _, test := range []struct {
		name  string
		cause error
		want  string
	}{
		{"dial", &net.OpError{Op: "dial", Err: errors.New("refused")}, "Service did not accept a connection"},
		{"timeout", &net.OpError{Op: "read", Err: &net.DNSError{IsTimeout: true}}, "Check timed out"},
		{"read", &net.OpError{Op: "read", Err: errors.New("closed")}, "Service could not be reached"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range []error{test.cause, fmt.Errorf("transport: %w", test.cause), &url.Error{Op: "Get", URL: "http://local", Err: fmt.Errorf("transport: %w", test.cause)}} {
				if got := explainProbeError(err); got != test.want {
					t.Fatalf("error %v translated to %q, want %q", err, got, test.want)
				}
			}
		})
	}
}
