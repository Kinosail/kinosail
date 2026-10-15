package configurationcore

import (
	"maps"
	"strings"
)

type RetiredPrefixes []string

func (prefixes RetiredPrefixes) Retired(key string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func (prefixes RetiredPrefixes) Discard(values map[string]string) {
	maps.DeleteFunc(values, func(key, _ string) bool { return prefixes.Retired(key) })
}
