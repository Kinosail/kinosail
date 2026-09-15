package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/credentials"
)

var (
	errInvalidOwnerSetup = errors.New("name and a secure 12-character password are required")
	errSetupComplete     = errors.New("server setup is already complete")
)

func (auth *authentication) createFirstOwner(name, password string, totp, automaticUpdates bool) (viewerProfile, *mfaEnrollment, error) {
	auth.setupMu.Lock()
	defer auth.setupMu.Unlock()
	if auth.profiles.hasProfiles() {
		return viewerProfile{}, nil, errSetupComplete
	}
	return auth.createOwner(name, password, totp, automaticUpdates)
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

func (auth *authentication) createOwner(name, password string, totp, automaticUpdates bool) (viewerProfile, *mfaEnrollment, error) {
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
	if auth.profiles.addOwner(profile) != nil {
		return viewerProfile{}, nil, errors.New("could not create Owner Profile")
	}
	if err := auth.settings.setOnboardingPending(true); err != nil {
		return viewerProfile{}, nil, errors.New("could not prepare onboarding")
	}
	if err := auth.settings.setUpdateChecks(automaticUpdates); err != nil {
		return viewerProfile{}, nil, errors.New("could not save update preference")
	}
	return profile, enrollment, nil
}
