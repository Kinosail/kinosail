package server

import settingsops "github.com/MikeO7/kinosail/packages/settings"

func (store *settingsStore) libraryFolders() settingsops.LibraryFolders {
	return settingsops.NewLibraryFolders(store.mediaRoot, libraryFolderState{store})
}

func (store *settingsStore) roots() []libraryRoot { return store.libraryFolders().Roots() }

func (store *settingsStore) add(path string) error { return store.libraryFolders().Add(path) }

func (store *settingsStore) remove(path string) error { return store.libraryFolders().Remove(path) }
