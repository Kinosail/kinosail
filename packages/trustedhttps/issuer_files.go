package trustedhttps

import "github.com/MikeO7/kinosail/packages/privatefile"

func writePrivate(path string, contents []byte) error {
	return privatefile.Write(path, contents)
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	return privatefile.Read(path, limit)
}

func certificateChainSize(chain [][]byte) int {
	size := 0
	for _, certificate := range chain {
		size += len(certificate)
	}
	return size
}
