//go:build !linux

package downloads

import (
	"errors"
	"os"
)

func cloneFile(*os.File, *os.File) error {
	return errors.ErrUnsupported
}
