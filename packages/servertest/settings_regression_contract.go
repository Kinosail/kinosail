package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// SettingsFixture preserves each real server's local test configuration.
type SettingsFixture struct{ MediaDir, DataDir, CacheDir, FFmpeg, FFprobe string }

// SettingsRegression binds common settings scenarios to the app's real server.
type SettingsRegression struct {
	New            func(SettingsFixture) http.Handler
	PlayableHLS    func() string
	APICall        func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder
	AssertBody     func(*testing.T, *httptest.ResponseRecorder, int, ...string)
	ProtectionText string
}

// RunSettings verifies the shared playback and server settings workflows.
func RunSettings(t *testing.T, suite SettingsRegression) {
	t.Run("OwnerCanPreferHighQualityTranscoding", suite.OwnerCanPreferHighQualityTranscoding)
	t.Run("TranscoderDefaultsToRecommendedAutomaticSelections", suite.TranscoderDefaultsToRecommendedAutomaticSelections)
	t.Run("PlaybackSettingsExplainTheDirectFirstDefault", suite.PlaybackSettingsExplainTheDirectFirstDefault)
	t.Run("SettingsDoNotExposeRetiredCloudBroker", suite.SettingsDoNotExposeRetiredCloudBroker)
	t.Run("OwnerCanChooseCompatiblePlaybackByDefault", suite.OwnerCanChooseCompatiblePlaybackByDefault)
	t.Run("OwnerCanAutoplayTheNextEpisode", suite.OwnerCanAutoplayTheNextEpisode)
	t.Run("OwnerCanDisableSubtitlesByDefault", suite.OwnerCanDisableSubtitlesByDefault)
	t.Run("PlaybackModeRejectsInvalidValue", suite.PlaybackModeRejectsInvalidValue)
	t.Run("OwnerCanNameTheirServer", suite.OwnerCanNameTheirServer)
}

// RunLibrarySettings verifies library persistence, boundaries, and diagnostics.
func RunLibrarySettings(t *testing.T, suite SettingsRegression) {
	t.Run("OwnerCanOpenLibrarySettings", suite.OwnerCanOpenLibrarySettings)
	t.Run("SettingsShowLibraryDiagnostics", suite.SettingsShowLibraryDiagnostics)
	t.Run("OwnerCanPersistALibraryFolder", suite.OwnerCanPersistALibraryFolder)
	t.Run("LibraryFolderCannotEscapeMediaMount", suite.LibraryFolderCannotEscapeMediaMount)
	t.Run("LibrarySettingPathsRejectAmbiguousInputWithoutSideEffects", suite.LibrarySettingPathsRejectAmbiguousInputWithoutSideEffects)
	t.Run("OwnerCanRemoveALibraryFolder", suite.OwnerCanRemoveALibraryFolder)
}
