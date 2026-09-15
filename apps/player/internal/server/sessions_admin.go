package server

import "net/http"

func (store *profileStore) activeSessions() int { return store.sessionModule().Active() }

func (store *profileStore) revokeOtherSessions(request *http.Request) error {
	return store.sessionModule().RevokeOthers(sessionToken(request))
}

func (store *profileStore) devices() []deviceSession { return store.sessionModule().Devices() }

func (store *profileStore) revokeDevice(id string) error {
	return store.sessionModule().RevokeDevice(id)
}
