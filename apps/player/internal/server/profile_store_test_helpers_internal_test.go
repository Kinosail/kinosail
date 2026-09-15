package server

func addTestOwner(store *profileStore, profile viewerProfile) error {
	profiles := []viewerProfile{profile}
	if err := store.save(profiles); err != nil {
		return err
	}
	store.profiles = profiles
	return nil
}
