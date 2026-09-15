package server_test

import "testing"

func TestIdentityAuditPersistsWithoutSecrets(t *testing.T) {
	auditFixture.IdentityAuditPersistsWithoutSecrets(t)
}

func TestOwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t *testing.T) {
	auditFixture.OwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t)
}

func TestActivityJournalSurvivesRestartAndRecoveryBackup(t *testing.T) {
	auditFixture.ActivityJournalSurvivesRestartAndRecoveryBackup(t)
}
