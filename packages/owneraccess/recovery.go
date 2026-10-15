package owneraccess

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func Recover(directory string) error {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return errors.New("invalid Server data directory")
	}
	if err := privatefile.Create(filepath.Join(directory, "owner-access.disabled")); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.New("could not disable management")
	}
	return privatefile.Write(filepath.Join(directory, "owner-access.json"), []byte(`{"enabled":false,"endpoint":"","privateKey":"","devices":[]}`))
}
