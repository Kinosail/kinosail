package server

import (
	"github.com/MikeO7/kinosail/packages/httpguard"
)

func httpguardTemplateSource(source string) string {
	return httpguard.CSRFTemplateSource(source)
}
