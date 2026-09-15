package identitycore

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/MikeO7/kinosail/packages/credentials"
	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	householdProfileError = "invalid household profile"
	householdAccessError  = "household profiles must be Viewers with library access"
	householdRatingError  = "invalid household content access"
	parentRatingError     = "Parent profiles must allow all ratings"
	profileInputError     = "name and a secure 12-character password are required"
)

var householdProfileKeys = []string{"name", "password", "owner", "rating", "libraries", "start", "end", "remote", "transcode", "downloads", "kind"}

// OnboardingViewerHandler applies Player's household profile policy before app persistence.
func OnboardingViewerHandler(add func(string, string, bool, ProfilePolicy) (string, error), writeError func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	if add == nil {
		return unavailableViewerMutationHandler()
	}
	return viewerMutationHandler(householdProfileKeys, "/onboarding/household", func(values url.Values) (string, error) {
		name, password, policy, message := onboardingViewerInput(values)
		if message != "" {
			return message, nil
		}
		if _, err := add(name, password, false, policy); err != nil {
			return "", err
		}
		return "", nil
	}, writeError)
}

func onboardingViewerInput(values url.Values) (string, string, ProfilePolicy, string) { //nolint:cyclop // This is the complete household profile invariant boundary.
	kind, validKind := httpguard.RequiredValue(values, "kind", 6)
	if !validKind || kind != "parent" && kind != "child" {
		return "", "", ProfilePolicy{}, householdProfileError
	}
	owner, validOwner := httpguard.RequiredValue(values, "owner", 5)
	libraries, validLibraries := httpguard.RequiredValue(values, "libraries", 3)
	if !validOwner || owner != "false" || !validLibraries || libraries != "all" {
		return "", "", ProfilePolicy{}, householdAccessError
	}
	rating, validRating := httpguard.RequiredValue(values, "rating", 6)
	if !validRating || rating != "family" && rating != "teen" && rating != "all" {
		return "", "", ProfilePolicy{}, householdRatingError
	}
	if kind == "parent" && rating != "all" {
		return "", "", ProfilePolicy{}, parentRatingError
	}
	name := strings.TrimSpace(values.Get("name"))
	password, validPassword := httpguard.RequiredValue(values, "password", viewerPasswordLimit)
	if name == "" || len(name) > maxNameLength || !validPassword || credentials.Validate(password) != nil {
		return "", "", ProfilePolicy{}, profileInputError
	}
	policy, validPolicy := onboardingViewerPolicy(values, rating)
	if !validPolicy {
		return "", "", ProfilePolicy{}, invalidViewerProfile
	}
	if err := policy.Valid(); err != nil {
		return "", "", ProfilePolicy{}, err.Error()
	}
	return name, password, policy, ""
}

func onboardingViewerPolicy(values url.Values, rating string) (ProfilePolicy, bool) {
	start, validStart := httpguard.OptionalValue(values, "start", 5)
	end, validEnd := httpguard.OptionalValue(values, "end", 5)
	remote, validRemote := onboardingViewerFlag(values, "remote")
	transcode, validTranscode := onboardingViewerFlag(values, "transcode")
	downloads, validDownloads := onboardingViewerFlag(values, "downloads")
	policy := ProfilePolicy{Downloads: downloads, Transcode: transcode, Remote: remote, Rating: rating, Libraries: []string{"all"}, AccessStart: strings.TrimSpace(start), AccessEnd: strings.TrimSpace(end)}
	return policy, validStart && validEnd && validRemote && validTranscode && validDownloads
}

func onboardingViewerFlag(values url.Values, key string) (bool, bool) {
	if _, present := values[key]; !present {
		return false, true
	}
	value, valid := httpguard.RequiredValue(values, key, 4)
	return value == "true", valid && value == "true"
}
