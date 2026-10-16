package server_test

import "testing"

func TestOwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t *testing.T) {
	auditFixture.OwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t)
}

func TestActivityJournalSurvivesRestartAndStaysOutOfPortableBackup(t *testing.T) {
	auditFixture.ActivityJournalSurvivesRestartAndStaysOutOfPortableBackup(t)
}

func TestRepeatedDenialsFromOneSourceStayBounded(t *testing.T) {
	auditFixture.RepeatedDenialsFromOneSourceStayBounded(t)
}
