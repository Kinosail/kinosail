package server

import (
	"io"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func decodeExternalJSON(reader io.Reader, maximum int64, target any) error {
	return httpguard.DecodeJSON(reader, maximum, target, false)
}
