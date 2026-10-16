package server_test

import "testing"

func TestOwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t *testing.T) {
	auditFixture.OwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t)
}

func TestActivityJournalSurvivesRestartAndRecoveryBackup(t *testing.T) {
	auditFixture.ActivityJournalSurvivesRestartAndRecoveryBackup(t)
}
