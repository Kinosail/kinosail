//go:build !windows

package auditjournal

import "os"

func securePermissions(info os.FileInfo) bool { return info.Mode().Perm()&0o077 == 0 }
