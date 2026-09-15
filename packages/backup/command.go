package backup

import (
	"errors"
	"io"
)

const commandUsage = "usage: kinosail backup [verify] | kinosail restore"

// Command runs Player's recovery command contract.
type Command func([]string, io.Reader, io.Writer, string, string) (bool, error)

// BindCommand binds recovery command orchestration to one app backup service.
func BindCommand(service *Service, version func() string) Command {
	if service == nil || version == nil {
		return func([]string, io.Reader, io.Writer, string, string) (bool, error) {
			return true, errors.New("backup command is not configured")
		}
	}
	return func(args []string, input io.Reader, output io.Writer, dataDir, key string) (bool, error) {
		return service.command(args, input, output, dataDir, key, version)
	}
}

func (service *Service) command(args []string, input io.Reader, output io.Writer, dataDir, key string, version func() string) (bool, error) { //nolint:cyclop // The bounded command matrix remains below the repository complexity ceiling.
	if len(args) == 0 || args[0] != "backup" && args[0] != "restore" {
		return false, nil
	}
	if len(args) == 1 && args[0] == "backup" {
		if output == nil {
			return true, errors.New("backup output is unavailable")
		}
		return true, service.WriteAuto(output, dataDir, key, version())
	}
	if len(args) == 2 && args[0] == "backup" && args[1] == "verify" {
		if input == nil {
			return true, errors.New("backup input is unavailable")
		}
		return true, service.VerifyAuto(input, key)
	}
	if len(args) == 1 {
		if input == nil {
			return true, errors.New("backup input is unavailable")
		}
		return true, service.RestoreAuto(input, dataDir, key)
	}
	return true, errors.New(commandUsage)
}
