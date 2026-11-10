package identitycore

import "errors"

var errInvalidCredentials = errors.New("invalid credentials")

func passwordLogin[T any](name, password string, authenticate func(string, string) (T, bool), signIn func(T) error) (T, error) {
	var zero T
	if authenticate == nil || signIn == nil {
		return zero, errInvalidConfig
	}
	profile, valid := authenticate(name, password)
	if !valid {
		return zero, errInvalidCredentials
	}
	if err := signIn(profile); err != nil {
		return zero, err
	}
	return profile, nil
}
