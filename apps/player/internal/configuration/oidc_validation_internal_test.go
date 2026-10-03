package configuration

import (
	"strings"
	"testing"
)

func TestValidRemoteTokenRejectsWrongLengthAndCharacters(t *testing.T) {
	valid := "abcdefghijklmnopqrstuvwxyz012345"
	for token, want := range map[string]bool{
		valid:                    true,
		valid + "-_:":            false,
		"short":                  false,
		strings.Repeat("a", 129): false,
	} {
		if got := validRemoteToken(token); got != want {
			t.Errorf("validRemoteToken(%q) = %t, want %t", token, got, want)
		}
	}
}
