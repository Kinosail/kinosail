package server

import (
	"context"
	"io"
	"net/http"

	"github.com/MikeO7/kinosail/packages/homeassistant"
)

const homeAssistantPairingHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#0b0d0b"><title>Pair Home Assistant · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-2"></head><body class="settings-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/settings#access">{{icon "back"}} Settings</a><header class="settings-intro"><span class="eyebrow">Local pairing · 10 minutes</span><h1>Connect Home Assistant</h1><p>In Home Assistant, add the Kinosail integration. Enter this Server address and the one-time code.</p></header><section class="wide pairing-code"><h2>Pairing code</h2><code class="grant-link">{{.}}</code><p>This code creates one revocable Home Assistant connection. It does not expose an Owner session or your media files.</p></section><p><a class="mode" href="/settings#access">Done</a></p></main></body></html>`

var homeAssistantPairingView = newLocalizedTemplate("home-assistant-pairing", homeAssistantPairingHTML)

type homeAssistantIntegration struct {
	*homeassistant.Integration[viewerProfile]
}

func homeAssistantProfile(profile viewerProfile) homeassistant.Profile[viewerProfile] {
	return homeassistant.Profile[viewerProfile]{Source: profile, ID: profile.ID, Name: profile.Name, Owner: profile.Owner, APIKey: profile.APIKey, Scopes: profile.Scopes}
}

func newHomeAssistant(settings *settingsStore, profiles *profileStore, index *libraryIndex, progress *progressStore, lists *listStore, lifecycle context.Context, authURL string, random io.Reader) (*homeAssistantIntegration, error) {
	integration, err := homeassistant.New(homeassistant.Config[viewerProfile]{
		Lifecycle:   lifecycle,
		AuthURL:     authURL,
		Random:      random,
		Enabled:     settings.homeAssistant,
		SaveEnabled: settings.setHomeAssistant,
		RevokeKeys:  profiles.revokeHomeAssistantKeys,
		CreateKey:   profiles.createHomeAssistantKey,
		FindProfile: func(id string) (homeassistant.Profile[viewerProfile], bool) {
			profile, found := profiles.byID(id)
			return homeAssistantProfile(profile), found
		},
		CurrentProfile: func(request *http.Request) homeassistant.Profile[viewerProfile] {
			return homeAssistantProfile(currentViewer(request))
		},
		Server: func() homeassistant.Server {
			return homeassistant.Server{Name: settings.serverName(), ID: settings.jellyfinID()}
		},
		TrustedHTTPS: func() homeassistant.TrustedHTTPS {
			trusted := settings.trustedHTTPS()
			return homeassistant.TrustedHTTPS{Hostname: trusted.Hostname, Configured: trusted.Configured}
		},
		Browse: func(request *http.Request) (homeassistant.Library, error) {
			page, browseErr := browseLibrary(request, index, progress, lists)
			if browseErr != nil {
				return homeassistant.Library{}, browseErr
			}
			viewer := currentViewer(request)
			items := make([]clientItem, 0, len(page.Items))
			for _, item := range page.Items {
				items = append(items, toClientItem(request, progress, viewer, item))
			}
			return homeassistant.Library{Items: items, View: page.View, Total: page.Total, Offset: page.Offset, Limit: page.Limit}, nil
		},
		VisibleItem: func(request *http.Request, id string) (homeassistant.Item, bool) {
			item, found := visibleItem(request, index, id)
			return homeassistant.Item{ID: item.ID, Path: item.Path}, found
		},
		FindItem: func(id string) (homeassistant.Item, bool) {
			item, found := index.Find(id)
			return homeassistant.Item{ID: item.ID, Path: item.Path}, found
		},
		SafePath: index.Safe,
	})
	if err != nil {
		return nil, err
	}
	return &homeAssistantIntegration{integration}, nil
}

func (store *settingsStore) homeAssistant() bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.HomeAssistant
}

func (store *settingsStore) setHomeAssistant(enabled bool) error {
	if err := store.editable("integrations.home_assistant.enabled"); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.HomeAssistant = enabled
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func saveHomeAssistant(integration *homeAssistantIntegration, destination string) http.HandlerFunc {
	return integration.SaveSetting(destination, localizedError)
}

func (integration *homeAssistantIntegration) pairingPage() http.HandlerFunc {
	return integration.PairingHandler(
		func(writer http.ResponseWriter, request *http.Request, code string) error {
			return homeAssistantPairingView.Execute(writer, request, code)
		},
		localizedError,
		localizedNotFound,
	)
}
