package identitycore

import "github.com/MikeO7/kinosail/packages/library"

// LibraryPolicy creates one Viewer's content access policy.
func LibraryPolicy(profile Profile) library.Policy {
	return library.NewPolicy(profile.Owner, profile.Rating, profile.Libraries)
}

// VisibleLibrary returns the current items allowed for one Viewer.
func VisibleLibrary(index library.VisibilityIndex, profile Profile) ([]library.Item, error) {
	return library.Visible(index, LibraryPolicy(profile).Allows)
}

// VisibleItem resolves one safe item allowed for one Viewer.
func VisibleItem(index library.VisibilityIndex, profile Profile, id string) (library.Item, bool) {
	return library.VisibleItem(index, id, LibraryPolicy(profile).Allows)
}

// CanView reports whether one Viewer can access an item.
func CanView(profile Profile, item library.Item) bool { return LibraryPolicy(profile).Allows(item) }
