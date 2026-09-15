package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/credentials"
)

var (
	errInvalidOwnerSetup = errors.New("Choose a name and a password with at least 12 characters.") //nolint:staticcheck // ST1005: preserve the existing user-facing setup guidance and localization contract.
	errSetupComplete     = errors.New("server setup is already complete")
)

func (auth *authentication) createFirstOwner(ctx context.Context, name, password string, totp, automaticUpdates bool) (viewerProfile, *mfaEnrollment, error) {
	auth.setupMu.Lock()
	defer auth.setupMu.Unlock()
	if auth.profiles.hasProfiles() {
		return viewerProfile{}, nil, errSetupComplete
	}
	return auth.createOwner(ctx, name, password, totp, automaticUpdates)
}

func ownerSetupStatus(err error) int {
	switch {
	case errors.Is(err, errInvalidOwnerSetup):
		return http.StatusBadRequest
	case errors.Is(err, errSetupComplete):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func (auth *authentication) createOwner(ctx context.Context, name, password string, totp, automaticUpdates bool) (viewerProfile, *mfaEnrollment, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 || credentials.Validate(password) != nil {
		return viewerProfile{}, nil, errInvalidOwnerSetup
	}
	profile, err := newProfile(name, password, true)
	if err != nil {
		return viewerProfile{}, nil, errors.New("could not create Owner Profile")
	}
	var enrollment *mfaEnrollment
	if totp {
		value, err := auth.mfa.setup(profile)
		if err != nil {
			return viewerProfile{}, nil, err
		}
		enrollment = &value
	}
	if err := auth.commitFirstOwner(ctx, profile, automaticUpdates); err != nil {
		if enrollment != nil {
			auth.mfa.discard(profile.ID)
		}
		if errors.Is(err, errSetupComplete) {
			return viewerProfile{}, nil, err
		}
		return viewerProfile{}, nil, errors.New("could not create Owner Profile")
	}
	return profile, enrollment, nil
}

func (auth *authentication) commitFirstOwner(ctx context.Context, profile viewerProfile, automaticUpdates bool) error {
	auth.settings.mu.Lock()
	defer auth.settings.mu.Unlock()
	auth.profiles.mu.Lock()
	defer auth.profiles.mu.Unlock()
	if len(auth.profiles.profiles) != 0 {
		return errSetupComplete
	}
	settings := auth.settings.value
	settings.OnboardingPending, settings.UpdateChecks = true, automaticUpdates
	profiles := []viewerProfile{profile}
	related := authRelatedDocument{"profiles.json", auth.profiles.file}
	if err := auth.persistSettingsAnd(ctx, settings, related, profiles); err != nil {
		return err
	}
	auth.settings.value, auth.profiles.profiles = settings, profiles
	return nil
}
