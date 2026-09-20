package remoteaccess

import (
	"context"
	"strings"
	"testing"
)

func TestUpdateDNSRejectsInvalidAndCanceledRequests(t *testing.T) {
	t.Parallel()
	if err := UpdateDNS(t.Context(), "invalid/name", "token"); err == nil {
		t.Fatal("invalid DNS domain accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := UpdateDNS(ctx, "family", strings.Repeat("a", 32)); err == nil {
		t.Fatal("canceled DNS update accepted")
	}
}
