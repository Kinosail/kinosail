package server_test

import "testing"

func TestLibraryPaginationIsSharedByAPIAndWeb(t *testing.T) {
	libraryAPIFixture.LibraryPaginationIsSharedByAPIAndWeb(t)
}

func TestDeepLibraryPageDoesNotRepeatHomeShelves(t *testing.T) {
	libraryAPIFixture.DeepLibraryPageDoesNotRepeatHomeShelves(t)
}

func TestInfiniteLibraryPageReturnsOnlyTheBoundedFragment(t *testing.T) {
	libraryAPIFixture.InfiniteLibraryPageReturnsOnlyTheBoundedFragment(t)
}

func TestLibraryPaginationRejectsAmbiguousAndOutOfRangeInput(t *testing.T) {
	libraryAPIFixture.LibraryPaginationRejectsAmbiguousAndOutOfRangeInput(t)
}
