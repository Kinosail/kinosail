package identitycore

import "github.com/MikeO7/kinosail/packages/library"

func LibraryPolicy(profile Profile) library.Policy {
	return library.NewPolicy(profile.Owner, profile.Rating, profile.Libraries)
}

func VisibleView(index library.VisibilityIndex, profile Profile) (*library.View, error) {
	return index.View(LibraryPolicy(profile))
}

func VisibleLibrary(index library.VisibilityIndex, profile Profile) ([]library.Item, error) {
	view, err := VisibleView(index, profile)
	return view.Items(), err
}

func VisibleItem(index library.VisibilityIndex, profile Profile, id string) (library.Item, bool) {
	return library.VisibleItem(index, id, LibraryPolicy(profile).Allows)
}

func CanView(profile Profile, item library.Item) bool { return LibraryPolicy(profile).Allows(item) }
