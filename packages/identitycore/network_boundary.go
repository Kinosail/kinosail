package identitycore

import (
	"net"
	"net/http"
	"net/netip"
)

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
