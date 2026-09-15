package dashboard

import (
	"context"
	"testing"
)

func TestYAMLImportRejectsAmbiguityBeforeServiceAccess(t *testing.T) {
	good := "apps: [{name: media, url: https://media.example}]"
	for _, content := range []string{good + "\n---\n" + good, good + "\n---", good + "\napps: []", "apps: &a [*a]"} {
		// A nil service proves invalid input returns before clock, store or state access.
		var service *Service
		if _, err := service.ImportExternal(context.Background(), ExternalImport{Source: "homarr", Content: content}, 0, "owner"); err == nil {
			t.Fatal("accepted ambiguous YAML")
		}
	}
}
