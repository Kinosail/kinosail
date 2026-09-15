package configuration

import sharedvalidation "github.com/MikeO7/kinosail/packages/validation"

func validateSupporter(configured Snapshot) error {
	return sharedvalidation.ValidateSupporter(configured.String("supporter.activation_url"), configured.String("supporter.url"))
}
