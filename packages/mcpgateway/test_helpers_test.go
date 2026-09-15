package mcpgateway

import (
	"strings"
	"testing"
)

func htmlFormValue(t *testing.T, page, name string) string {
	t.Helper()
	marker := `name="` + name + `" value="`
	start := strings.Index(page, marker)
	if start < 0 {
		t.Fatalf("field %q missing from %q", name, page)
	}
	start += len(marker)
	end := strings.Index(page[start:], `"`)
	if end < 0 {
		t.Fatalf("field %q not closed", name)
	}
	return page[start : start+end]
}
