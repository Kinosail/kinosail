//go:build !windows

package database

import "syscall"

func enforcePrivateCreationMask() { syscall.Umask(0o077) }
