package server_test

import (
	"testing"
)

func TestInvalidDurableShapesFailApplicationClosed(t *testing.T) {
	libraryAPIFixture.InvalidDurableShapesFailClosed(t)
}

func TestVersionedAPISurfacesIndividualStateFileFailures(t *testing.T) {
	libraryAPIFixture.IndividualStateFileFailures(t, firstAPIItemID)
}
