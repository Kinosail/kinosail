package homeassistant

import (
	"strings"
	"testing"
)

func TestAdvertisementRejectsInvalidRecordsBeforeOpeningSockets(t *testing.T) {
	for _, name := range []string{"", " padded", "control\n", strings.Repeat("x", 64), string([]byte{255})} {
		if service, err := advertiseHomeAssistant(name, 38127, []string{"id=test"}); err == nil || service != nil {
			t.Fatalf("accepted name %q", name)
		}
	}
	for _, port := range []int{-1, 0, 65536} {
		if service, err := advertiseHomeAssistant("test", port, []string{"id=test"}); err == nil || service != nil {
			t.Fatalf("accepted port %d", port)
		}
	}
	for _, text := range [][]string{nil, {"id="}, {"id=a", "id=b"}, {"unknown=value"}, {"id"}, {"version=1"}, {"id=a", "tls=yes"}, {"id=a", "version=2"}, {"id=a", "url=javascript:bad"}, {"id=a", "url=https://user:secret@example.test"}, {"id=a", "url=http://"}, {"id=a", "url=%"}, {"id=" + strings.Repeat("x", 253)}, {"id=a\n"}, {"id=a", "version=1", "tls=true", "verify_ssl=true", "url=https://example.test", "extra=x"}} {
		if service, err := advertiseHomeAssistant("test", 38127, text); err == nil || service != nil {
			t.Fatalf("accepted TXT %q", text)
		}
	}
}

func TestAdvertisementAcceptsBoundaryRecords(t *testing.T) {
	for _, port := range []int{1, 65535} {
		for _, name := range []string{"x", strings.Repeat("x", 63), strings.Repeat("é", 31) + "x"} {
			for _, text := range [][]string{
				{"id=" + strings.Repeat("x", 252)},
				{"id=test", "version=1", "tls=false", "verify_ssl=false", "url=http://example.test"},
				{"id=test", "version=1", "tls=true", "verify_ssl=true", "url=https://example.test"},
			} {
				if err := validateAdvertisement(name, port, text); err != nil {
					t.Fatalf("rejected valid name %q, port %d, TXT %q: %v", name, port, text, err)
				}
			}
		}
	}
}
