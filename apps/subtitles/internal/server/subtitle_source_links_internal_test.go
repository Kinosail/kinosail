package server

import "testing"

func TestTrustedProviderLinkAcceptsOnlyTheProviderOrItsHTTPSDomain(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		base, link, domain string
		trusted            bool
	}{
		{"https://api.subdl.com/api/v1", "https://api.subdl.com/subtitle/1.zip", "subdl.com", true},
		{"https://api.subdl.com/api/v1", "http://api.subdl.com/subtitle/1.zip", "subdl.com", false},
		{"https://api.subdl.com/api/v1", "https://dl.subdl.com/subtitle/1.zip", "subdl.com", true},
		{"https://api.subdl.com/api/v1", "https://subdl.com.evil.example/1.zip", "subdl.com", false},
		{"https://api.subdl.com/api/v1", "http://dl.subdl.com/1.zip", "subdl.com", false},
		{"https://api.opensubtitles.com/api/v1", "https://opensubtitles.com/file", "opensubtitles.com", true},
		{"https://api.subsource.net/api/v1", "https://cdn.subsource.net/file", "", false},
		{"http://127.0.0.1:9000/api/v1", "http://127.0.0.1:9000/file", "", true},
		{"https://api.subdl.com/api/v1", "not a url", "subdl.com", false},
		{"https://api.subdl.com/api/v1", "/relative", "subdl.com", false},
	} {
		if got := trustedProviderLink(test.base, test.link, test.domain); got != test.trusted {
			t.Errorf("trustedProviderLink(%q, %q, %q) = %t", test.base, test.link, test.domain, got)
		}
	}
}
