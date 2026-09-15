package server

import "github.com/MikeO7/kinosail/packages/privatefile"

func writeAtomicFile(target string, data []byte) error {
	return privatefile.WriteCache(target, data)
}
