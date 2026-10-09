package playback

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// The P2 envelope retains whole-policy equality in every binding and master.
// Only this ordered canonical suffix may extend the unchanged base grammar.
func copiedAACPolicyParts(policy string) (string, string, bool) {
	if !strings.Contains(policy, ":copied-source=") && !strings.Contains(policy, ":copied-aac=") {
		return policy, "", true
	}
	base, extension, found := strings.Cut(policy, ":copied-source=")
	if !found || strings.Contains(base, ":copied-aac=") || !strings.HasSuffix(base, ":hls=15") {
		return "", "", false
	}
	token, version, found := strings.Cut(extension, ":")
	if !found || version != "copied-aac=2" || !canonicalCopiedAACSource(token) {
		return "", "", false
	}
	return base, token, true
}

func canonicalCopiedAACSource(value string) bool {
	device, inode, found := strings.Cut(value, ".")
	if !found || len(device) == 0 || len(device) > 16 || len(inode) == 0 || len(inode) > 16 {
		return false
	}
	dev, devErr := strconv.ParseUint(device, 16, 64)
	ino, inoErr := strconv.ParseUint(inode, 16, 64)
	return devErr == nil && inoErr == nil && ino != 0 && strconv.FormatUint(dev, 16) == device && strconv.FormatUint(ino, 16) == inode
}

func hlsSourceState(source, policy string) (string, error) {
	info, err := os.Stat(source) //nolint:gosec // Source comes from the scanned library.
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return "", errors.New("HLS source is not a regular media file")
	}
	_, token, valid := copiedAACPolicyParts(policy)
	if !valid || token != "" && !copiedAACSourceMatches(info, token) {
		return "", ErrHLSSourceChanged
	}
	return strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10), nil
}
