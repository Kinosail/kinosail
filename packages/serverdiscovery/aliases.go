package serverdiscovery

import (
	"net"
	"net/url"
	"strconv"
)

// LocalAliases accepts the exact local addresses resolved by Android NSD when
// a loopback-configured server has no externally configured connection URL.
func LocalAliases(origin string) []string {
	alias := LoopbackAlias(origin)
	if alias == "" {
		return nil
	}
	addresses, _ := net.InterfaceAddrs()
	return localAliases(origin, alias, addresses)
}

func localAliases(origin, hostname string, addresses []net.Addr) []string {
	port, txt, err := record("Player", origin)
	if err != nil || len(txt) != 2 || !validHostname(hostname) {
		return nil
	}
	parsed, _ := url.Parse(origin)
	aliases := []string{}
	add := func(host string) { aliases = append(aliases, aliasHosts(host, parsed.Scheme, port)...) }
	add(hostname)
	if len(addresses) > 256 {
		return aliases
	}
	seen := map[string]bool{}
	for _, address := range addresses {
		ip := localAddressIP(address)
		if ip == nil || len(seen) >= 32 {
			continue
		}
		if !localAliasIP(ip) {
			continue
		}
		host := ip.String()
		if !seen[host] {
			add(host)
			seen[host] = true
		}
	}
	return aliases
}

func aliasHosts(host, scheme string, port int) []string {
	var aliases []string
	aliases = append(aliases, net.JoinHostPort(host, strconv.Itoa(port)))
	if scheme == "http" && port == 80 || scheme == "https" && port == 443 {
		if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
			host = "[" + host + "]"
		}
		aliases = append(aliases, host)
	}
	return aliases
}

func localAliasIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.To4() != nil && ip.IsLinkLocalUnicast()
}

func localAddressIP(address net.Addr) net.IP {
	network, ok := address.(*net.IPNet)
	if !ok || network == nil {
		return nil
	}
	return network.IP
}
