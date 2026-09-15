//go:build !linux || (!amd64 && !arm64)

package publicgateway

import "errors"

func restrictNetwork() error {
	return errors.New("the isolated public gateway requires a supported Linux container")
}
