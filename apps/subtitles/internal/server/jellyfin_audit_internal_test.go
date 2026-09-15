package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestJellyfinSessionAuditClassifiesAndRedactsCredentials(t *testing.T) {
	servertest.JellyfinSessionAuditClassifiesAndRedactsCredentials(t, auditAction, auditDetails)
}
