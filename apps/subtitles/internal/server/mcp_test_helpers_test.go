package server

import (
	"context"
	"strings"
	"testing"
)

func withViewer(ctx context.Context, profile viewerProfile) context.Context {
	return context.WithValue(ctx, viewerContextKey{}, profile)
}

func formValue(t *testing.T, page, name string) string {
	t.Helper()
	marker := `name="` + name + `" value="`
	start := strings.Index(page, marker)
	if start < 0 {
		t.Fatalf("form field %q missing from %q", name, page)
	}
	start += len(marker)
	end := strings.Index(page[start:], `"`)
	if end < 0 {
		t.Fatalf("form field %q was not closed", name)
	}
	return page[start : start+end]
}
