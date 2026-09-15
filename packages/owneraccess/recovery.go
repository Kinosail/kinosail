package owneraccess

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

// Recover disables management from the local CLI without needing a working login,
// device, or pairing file. A running service observes the marker within two seconds.
func Recover(directory string) error {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return errors.New("invalid Server data directory")
	}
	if err := privatefile.Create(filepath.Join(directory, "owner-access.disabled")); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.New("could not disable management")
	}
	return privatefile.Write(filepath.Join(directory, "owner-access.json"), []byte(`{"enabled":false,"endpoint":"","privateKey":"","devices":[]}`))
}
