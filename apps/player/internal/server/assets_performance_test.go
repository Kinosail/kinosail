package server_test

import "testing"

func TestLibraryPrioritizesVisibleArtworkWithoutLayoutShift(t *testing.T) {
	assetContracts.LibraryPrioritizesVisibleArtworkWithoutLayoutShift(t)
}

func TestVersionedStaticAssetsUseImmutableCaching(t *testing.T) {
	assetContracts.VersionedStaticAssetsUseImmutableCaching(t)
}

func TestPlayerScriptURLTracksServedContents(t *testing.T) {
	assetContracts.PlayerScriptURLTracksServedContents(t)
}
