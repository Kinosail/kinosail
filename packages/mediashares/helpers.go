package mediashares

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"
)

func (store *Store) secret() (string, error) {
	value := make([]byte, 32)
	_, err := io.ReadFull(store.random, value)
	return base64.RawURLEncoding.EncodeToString(value), err
}

func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func cleanDeviceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Web browser"
	}
	for marker, label := range map[string]string{"Firefox/": "Firefox", "Edg/": "Microsoft Edge", "Chrome/": "Chrome", "Safari/": "Safari"} {
		if strings.Contains(name, marker) {
			return label
		}
	}
	return name[:min(len(name), 80)]
}
