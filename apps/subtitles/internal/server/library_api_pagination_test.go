package server_test

import "testing"

func TestLibraryPaginationRejectsAmbiguousAndOutOfRangeInput(t *testing.T) {
	libraryAPIFixture.LibraryPaginationRejectsAmbiguousAndOutOfRangeInput(t)
}
