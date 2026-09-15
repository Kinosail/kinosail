package server

func (store *profileStore) scimProfiles() []viewerProfile {
	return store.profileModule().SCIMProfiles()
}

func (store *profileStore) scimProfile(id string) (viewerProfile, bool) {
	return store.profileModule().SCIMProfile(id)
}

func (store *profileStore) createSCIMProfile(input scimProfileInput) (viewerProfile, error) {
	return store.profileModule().CreateSCIMProfile(input)
}

func (store *profileStore) updateSCIMProfile(id string, input scimProfileInput, expected string) (viewerProfile, error) {
	return store.profileModule().UpdateSCIMProfile(id, input, expected)
}

func (store *profileStore) deleteSCIMProfile(id, expected string) error {
	return store.profileModule().DeleteSCIMProfile(id, expected)
}
