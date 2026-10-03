package server

import "time"

func publicSessionTimeoutHours(settings installationSettings) (float64, float64) {
	if settings.PublicSessionInactiveHours != 0 && settings.PublicSessionAbsoluteHours != 0 {
		return settings.PublicSessionInactiveHours, settings.PublicSessionAbsoluteHours
	}
	// Absent fields preserve legacy validation, even when inactivity exceeds the cap.
	inactive, absolute := sessionTimeoutHours(settings)
	return inactive, min(absolute, 8)
}

func (store *settingsStore) publicSessionTimeouts() (time.Duration, time.Duration) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	inactive, absolute := publicSessionTimeoutHours(store.value)
	return time.Duration(inactive * float64(time.Hour)), time.Duration(absolute * float64(time.Hour))
}

func (store *settingsStore) publicSessionLifetime() time.Duration {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return publicSessionLifetime(store.value)
}

func publicSessionLifetime(settings installationSettings) time.Duration {
	if settings.PublicSessionAbsoluteHours != 0 {
		return time.Duration(settings.PublicSessionAbsoluteHours * float64(time.Hour))
	}
	// Legacy issuance and its cookie always used eight hours, independently of validation.
	return 8 * time.Hour
}

func (auth *authentication) publicAuthenticationMaximumAge() time.Duration {
	if auth.settings.snapshot().PublicSessionAbsoluteHours == 0 {
		return publicSessionMaximumAge
	}
	_, absolute := auth.settings.publicSessionTimeouts()
	return absolute
}
