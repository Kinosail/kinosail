package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestVersionedAPIRoutesDefaultToSecure(t *testing.T) {
	servertest.APISecurityContract(t, apiServer)
}
