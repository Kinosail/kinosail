package server_test

import "testing"

func TestLibrarySettingsRegression(t *testing.T) {
	suite := settingsRegression()
	t.Run("SettingsShowLibraryDiagnostics", suite.SettingsShowLibraryDiagnostics)
	t.Run("OwnerCanPersistALibraryFolder", suite.OwnerCanPersistALibraryFolder)
	t.Run("LibraryFolderCannotEscapeMediaMount", suite.LibraryFolderCannotEscapeMediaMount)
	t.Run("LibrarySettingPathsRejectAmbiguousInputWithoutSideEffects", suite.LibrarySettingPathsRejectAmbiguousInputWithoutSideEffects)
	t.Run("OwnerCanRemoveALibraryFolder", suite.OwnerCanRemoveALibraryFolder)
}
