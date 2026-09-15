package servertest

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/mediashares"
)

// SecurityFuzz binds parser checks and per-input authorization fixtures once per app.
type SecurityFuzz struct {
	AllowedOutboundIP func(net.IP) bool
	UnsafeCrossOrigin func(*http.Request) bool
	NewAuthorization  RemoteAuthorizationFuzzFactory
}

func (suite SecurityFuzz) RemoteBoundaryParsers(f *testing.F) {
	for _, seed := range []string{"bytes=0-1", "bytes=-500", "bytes=1-", "bytes=1-0", "bytes=0-1,2-3", "", "\x00", "18446744073709551616"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		_ = httpguard.ValidPublicRange([]string{value})
		_ = mediashares.ValidSingleByteRange(value)
		_ = suite.AllowedOutboundIP(net.IP([]byte(value)))
	})
}

func (suite SecurityFuzz) ExactBrowserOrigin(f *testing.F) {
	f.Add("media.example", "https://media.example")
	f.Add("media.example:443", "https://media.example")
	f.Add("media.example", "https://evil.example")
	f.Add("[2001:db8::1]:443", "https://[2001:db8::1]")
	f.Fuzz(func(t *testing.T, host, origin string) {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.invalid/", nil)
		request.Host, request.TLS = host, &tls.ConnectionState{}
		request.Header.Set("Origin", origin)
		_ = suite.UnsafeCrossOrigin(request)
	})
}

func (suite SecurityFuzz) PersistedMediaShareState(f *testing.F) {
	f.Add([]byte(`{"shares":{},"sessions":{}}`))
	f.Add([]byte(`{"shares":{"bad":{}},"sessions":{}}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 256<<10 {
			t.Skip()
		}
		var state mediashares.State
		if json.Unmarshal(data, &state) == nil && mediashares.ValidState(state) {
			mediashares.Prune(&state, 1)
			if !mediashares.ValidState(state) {
				t.Fatal("pruning made a valid media share state invalid")
			}
		}
	})
}
