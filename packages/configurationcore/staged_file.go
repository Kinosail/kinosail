package configurationcore

import "os"

// StagedFile is the durability surface required by a configuration transaction.
type StagedFile interface {
	Name() string
	Chmod(os.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

// WriteStagedFile durably writes and closes one configuration transaction file.
func WriteStagedFile(file StagedFile, data []byte, mode os.FileMode) error {
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
