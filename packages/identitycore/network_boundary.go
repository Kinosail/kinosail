package identitycore

import (
	"net"
	"net/http"
	"net/netip"
)

// PrivateNetwork keeps direct public traffic on the public policy even if an Owner
// accidentally forwards the local port. Headers never establish network locality.
// LAN IPv4, IPv6 ULA/link-local, and loopback still require normal Owner authentication.
func PrivateNetwork(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		address, parseErr := netip.ParseAddr(host)
		if err != nil || parseErr != nil {
			http.Error(w, "request network is unavailable", http.StatusForbidden)
			return
		}
		ip := net.IP(address.WithZone("").AsSlice())
		if !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() {
			r = markRemote(r)
		}
		next.ServeHTTP(w, r)
	})
}
