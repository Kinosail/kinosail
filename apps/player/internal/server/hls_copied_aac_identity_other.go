//go:build !linux

package server

import "os"

func copiedAACSourceToken(_ os.FileInfo) (string, error) {
	return "", errCopiedHLSIndex
}
