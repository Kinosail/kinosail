package identitycore

import "errors"

var ErrInvalidCredentials = errors.New("invalid credentials")

// PasswordLogin authenticates before it commits a session side effect.
func PasswordLogin[T any](name, password string, authenticate func(string, string) (T, bool), signIn func(T) error) (T, error) {
	var zero T
	if authenticate == nil || signIn == nil {
		return zero, ErrInvalidConfig
	}
	profile, valid := authenticate(name, password)
	if !valid {
		return zero, ErrInvalidCredentials
	}
	if err := signIn(profile); err != nil {
		return zero, err
	}
	return profile, nil
}
