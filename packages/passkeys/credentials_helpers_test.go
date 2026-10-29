package passkeys

import "github.com/go-webauthn/webauthn/webauthn"

func ValidateCredentials(credentials []webauthn.Credential) error {
	if len(credentials) > MaxCredentials {
		return ErrLimit
	}
	seen := make(map[string]struct{}, len(credentials))
	for index := range credentials {
		if err := ValidateCredential(&credentials[index]); err != nil {
			return err
		}
		key := string(credentials[index].ID)
		if _, found := seen[key]; found {
			return ErrDuplicate
		}
		seen[key] = struct{}{}
	}
	return nil
}
