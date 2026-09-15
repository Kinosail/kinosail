package configuration

import "errors"

const (
	subSourceAPIKey      = "integrations.subsource.api_key"
	subSourcePersonalUse = "integrations.subsource.personal_use"
)

func validateSubSource(configured Snapshot) error {
	key := configured.String(subSourceAPIKey) != ""
	accepted := configured.Bool(subSourcePersonalUse)
	if key != accepted {
		return errors.New("SubSource requires an API key and personal-use acceptance together")
	}
	return nil
}

func isSubSourceKey(key string) bool {
	return key == subSourceAPIKey || key == subSourcePersonalUse
}
