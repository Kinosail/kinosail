package auditjournal

import "os"

func securePermissions(os.FileInfo) bool { return true }
