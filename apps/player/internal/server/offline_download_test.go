package server_test

import "testing"

func TestTranscodedDownloadBecomesIntegrityCheckedReadyOfflineFile(t *testing.T) {
	downloadFixture.TranscodedDownloadBecomesIntegrityCheckedReadyOfflineFile(t)
}

func TestRestartDoesNotTrustMissingOrChangedReadyDownload(t *testing.T) {
	downloadFixture.RestartDoesNotTrustMissingOrChangedReadyDownload(t)
}
