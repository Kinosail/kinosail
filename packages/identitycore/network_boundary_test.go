package identitycore

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrivateNetworkNeverTrustsClientHeaders(t *testing.T) {
	for _, test := range []struct {
		address string
		public  bool
	}{{"127.0.0.1:5000", false}, {"192.168.1.8:5000", false}, {"[fd00::2]:5000", false}, {"[fe80::2%en0]:5000", false}, {"10.91.0.2:5000", true}, {"198.51.100.2:5000", true}, {"[2001:db8::2]:5000", true}} {
		t.Run(test.address, func(t *testing.T) {
			r := httptest.NewRequest("GET", "https://server.example/settings", nil)
			r.RemoteAddr = test.address
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			r.Header.Set("X-Kinosail-Remote", "false")
			called := false
			PrivateNetwork(TrustedProxy("", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				if RemoteRequest(r) != test.public {
					t.Fatalf("public=%v", RemoteRequest(r))
				}
			}))).ServeHTTP(httptest.NewRecorder(), r)
			if !called {
				t.Fatal("valid address rejected")
			}
		})
	}
	r := httptest.NewRequest("GET", "https://server.example", nil)
	r.RemoteAddr = "not-an-address"
	called := false
	w := httptest.NewRecorder()
	PrivateNetwork(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(w, r)
	if called || w.Code != http.StatusForbidden {
		t.Fatal("invalid address reached application")
	}
}
