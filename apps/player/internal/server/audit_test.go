package server_test

import "testing"

func TestIdentityAuditPersistsWithoutSecrets(t *testing.T) {
	auditFixture.IdentityAuditPersistsWithoutSecrets(t)
}

func TestOwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t *testing.T) {
	auditFixture.OwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t)
}

func TestActivityJournalSurvivesRestartAndStaysOutOfPortableBackup(t *testing.T) {
	auditFixture.ActivityJournalSurvivesRestartAndStaysOutOfPortableBackup(t)
}

func TestRepeatedDenialsFromOneSourceStayBounded(t *testing.T) {
	auditFixture.RepeatedDenialsFromOneSourceStayBounded(t)
}

func TestUntrustedRequestTextStaysInActivityJournal(t *testing.T) {
	auditFixture.UntrustedRequestTextStaysInActivityJournal(t)
}
