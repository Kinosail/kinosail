package remoteaccess

import (
	"net"
	"strconv"
)

func validListen(address string) bool {
	if len(address) > 64 {
		return false
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || host != "" && host != "localhost" && net.ParseIP(host) == nil {
		return false
	}
	number, err := strconv.ParseUint(port, 10, 16)
	return err == nil && number > 0 && strconv.FormatUint(number, 10) == port
}

func validDomain(domain string) bool { //nolint:cyclop // The nested ASCII allowlist rejects Unicode lookalikes.
	if domain == "" || len(domain) > 63 || domain[0] == '-' || domain[len(domain)-1] == '-' {
		return false
	}
	for _, character := range domain {
		if character < 'a' || character > 'z' {
			if character < '0' || character > '9' {
				if character != '-' {
					return false
				}
			}
		}
	}
	return true
}

func validToken(token string) bool { //nolint:cyclop,gocognit // The nested ASCII allowlist rejects whitespace and Unicode lookalikes.
	if len(token) < 32 || len(token) > 128 {
		return false
	}
	for _, character := range token {
		if character < 'a' || character > 'z' {
			if character < 'A' || character > 'Z' {
				if character < '0' || character > '9' {
					if character != '-' && character != '_' {
						return false
					}
				}
			}
		}
	}
	return true
}
