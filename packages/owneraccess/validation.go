package owneraccess

import (
	"encoding/base64"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func eligible(p identitycore.Profile) bool {
	return p.Owner && !p.Disabled && !p.SCIMDeleted && p.Secured() && p.Revision > 0
}

func decodeKey(raw string) []byte {
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 || base64.StdEncoding.EncodeToString(b) != raw {
		return nil
	}
	return b
}

func validText(s string, maximum int) bool {
	return s != "" && len(s) <= maximum && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}
func validLabel(s string) bool { return validText(s, 320) && utf8.RuneCountInString(s) <= 80 }
func validID(s string) bool    { return validText(s, 128) && strings.IndexFunc(s, unicode.IsSpace) < 0 }

func validEndpoint(s string) bool {
	host, port, err := net.SplitHostPort(s)
	return err == nil && validPort(port, 1024) && validHostname(host) && s == net.JoinHostPort(host, port)
}

func validPort(port string, minimum int) bool {
	number, err := strconv.Atoi(port)
	return err == nil && number >= minimum && number <= 65535 && strconv.Itoa(number) == port
}

func validHostname(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		if strings.IndexFunc(label, invalidHostnameRune) >= 0 {
			return false
		}
	}
	return true
}

func invalidHostnameRune(c rune) bool {
	return (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-'
}

func validOrigin(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	port := u.Port()
	return u.Scheme == "https" && validHostname(u.Hostname()) && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && (port == "" || validPort(port, 1))
}

func validState(s state) bool {
	if !s.Enabled {
		return s.Endpoint == "" && s.PrivateKey == "" && len(s.Devices) == 0
	}
	return validEndpoint(s.Endpoint) && decodeKey(s.PrivateKey) != nil && validDevices(s.Devices)
}

func validDevices(devices []Device) bool {
	if len(devices) > maxDevices {
		return false
	}
	keys, addresses := map[string]bool{}, map[string]bool{}
	for _, d := range devices {
		if !validDevice(d) || keys[d.PublicKey] || addresses[d.Address] {
			return false
		}
		keys[d.PublicKey], addresses[d.Address] = true, true
	}
	return true
}

func validDevice(d Device) bool {
	return validLabel(d.Label) && validID(d.ProfileID) && d.Revision > 0 && decodeKey(d.PublicKey) != nil && decodeKey(d.PresharedKey) != nil && validDeviceAddress(d.Address)
}

func validDeviceAddress(address string) bool {
	ip := net.ParseIP(address).To4()
	return ip != nil && ip.String() == address && ip[0] == 10 && ip[1] == 92 && ip[2] == 0 && ip[3] >= 2 && ip[3] <= 254
}
