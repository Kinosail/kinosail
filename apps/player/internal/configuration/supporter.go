package configuration

import "github.com/MikeO7/kinosail/packages/configurationcore"

func validateSupporter(configured Snapshot) error {
	return configurationcore.ValidateSupporter(configured.String("supporter.activation_url"), configured.String("supporter.url"))
}
