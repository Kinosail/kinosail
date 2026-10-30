package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestCSRFContract(t *testing.T) {
	servertest.CSRF(t, security, httpguard.CSRFToken, newLocalizedTemplate)
}
