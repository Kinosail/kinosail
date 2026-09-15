package configuration

import "errors"

func validateSCIM(configured Snapshot) error {
	token, expires := configured.String("integrations.scim.token"), configured.String("integrations.scim.token_expires_at")
	if token == "" && expires != "" {
		return errors.New("integrations.scim.token is required when token expiration is configured")
	}
	if token != "" && expires == "" {
		return errors.New("integrations.scim.token_expires_at is required when SCIM is configured")
	}
	return nil
}
