package privatefile

import "os"

// Windows access is controlled by the inherited access control list, not Unix mode bits.
func securePermissions(os.FileInfo) bool { return true }

// Windows does not support syncing an open directory through os.File.
func syncDirectory(string) error { return nil }
