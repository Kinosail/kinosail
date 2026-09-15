package main

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

func parseProbeInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 30 * time.Second, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval < 15*time.Second || interval > time.Hour {
		return 0, errors.New("probe interval must be from 15s to 1h")
	}
	return interval, nil
}

func validHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !validHostnameLabel(label) {
			return false
		}
	}
	return true
}

func validHostnameLabel(label string) bool {
	if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, character := range label {
		if !validHostnameCharacter(character) {
			return false
		}
	}
	return true
}

func validHostnameCharacter(character rune) bool {
	return character == '-' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
}

func validateRuntimePaths(listen, dataDir string) error {
	if len(listen) > 256 {
		return errors.New("listen address is invalid")
	}
	_, port, err := net.SplitHostPort(listen)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("listen address is invalid")
	}
	if dataDir == "" || len(dataDir) > 4096 || dataDir == "/" {
		return errors.New("data directory is invalid")
	}
	return nil
}
