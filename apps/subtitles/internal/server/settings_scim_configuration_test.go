package server_test

import "testing"

func TestOwnerCanConfigureSCIMPairThroughAPIAndWeb(t *testing.T) {
	scimConfiguration.OwnerCanConfigureSCIMPairThroughAPIAndWeb(t)
}

func TestExpiredSCIMConfigurationKeepsOwnerRecoveryAvailable(t *testing.T) {
	scimConfiguration.ExpiredSCIMConfigurationKeepsOwnerRecoveryAvailable(t)
}
