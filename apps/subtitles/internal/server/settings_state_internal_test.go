package server

import (
	"io"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSettingsStateContracts(t *testing.T) {
	servertest.SettingsStateContracts(t, func() servertest.SettingsStateFixture {
		store := &settingsStore{}
		return servertest.NewSettingsStateFixture(&store.value, store.snapshot, settingsStateReferences, &store.file, &store.value.HomeAssistant, &store.persist, func(random io.Reader) (servertest.SettingsEnabler, error) {
			return newHomeAssistant(store, newProfileStore(""), &libraryIndex{}, nil, nil, nil, "", random)
		}, store.homeAssistant)
	})
}

func settingsStateReferences(value *installationSettings) servertest.SettingsStateReferences {
	return servertest.SettingsReferences(&value.Libraries, &value.AutoSkip, &value.Navigation, &value.Supporter)
}
