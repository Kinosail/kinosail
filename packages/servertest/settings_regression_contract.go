package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type SettingsFixture struct{ MediaDir, DataDir, CacheDir, FFmpeg, FFprobe string }

type SettingsRegression struct {
	New            func(SettingsFixture) http.Handler
	PlayableHLS    func() string
	APICall        func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder
	AssertBody     func(*testing.T, *httptest.ResponseRecorder, int, ...string)
	ProtectionText string
}

func RunSettings(t *testing.T, suite SettingsRegression) {
	t.Run("OwnerCanPreferHighQualityTranscoding", suite.OwnerCanPreferHighQualityTranscoding)
	t.Run("TranscoderDefaultsToRecommendedAutomaticSelections", suite.TranscoderDefaultsToRecommendedAutomaticSelections)
	t.Run("PlaybackSettingsExplainTheDirectFirstDefault", suite.PlaybackSettingsExplainTheDirectFirstDefault)
	t.Run("SettingsDoNotExposeRetiredCloudBroker", suite.SettingsDoNotExposeRetiredCloudBroker)
	t.Run("SettingsThankDependencies", suite.SettingsThankDependencies)
	t.Run("OwnerCanChooseCompatiblePlaybackByDefault", suite.OwnerCanChooseCompatiblePlaybackByDefault)
	t.Run("OwnerCanAutoplayTheNextEpisode", suite.OwnerCanAutoplayTheNextEpisode)
	t.Run("OwnerCanDisableSubtitlesByDefault", suite.OwnerCanDisableSubtitlesByDefault)
	t.Run("PlaybackModeRejectsInvalidValue", suite.PlaybackModeRejectsInvalidValue)
	t.Run("OwnerCanNameTheirServer", suite.OwnerCanNameTheirServer)
}

func RunLibrarySettings(t *testing.T, suite SettingsRegression) {
	t.Run("OwnerCanOpenLibrarySettings", suite.OwnerCanOpenLibrarySettings)
	t.Run("SettingsShowLibraryDiagnostics", suite.SettingsShowLibraryDiagnostics)
	t.Run("OwnerCanPersistALibraryFolder", suite.OwnerCanPersistALibraryFolder)
	t.Run("LibraryFolderCannotEscapeMediaMount", suite.LibraryFolderCannotEscapeMediaMount)
	t.Run("LibrarySettingPathsRejectAmbiguousInputWithoutSideEffects", suite.LibrarySettingPathsRejectAmbiguousInputWithoutSideEffects)
	t.Run("OwnerCanRemoveALibraryFolder", suite.OwnerCanRemoveALibraryFolder)
}
