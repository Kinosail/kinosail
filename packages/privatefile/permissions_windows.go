package privatefile

import "os"

func securePermissions(os.FileInfo) bool { return true }

func syncDirectory(string) error { return nil }
