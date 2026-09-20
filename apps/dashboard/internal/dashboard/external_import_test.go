package dashboard

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPreviewExternalSupportsDashboardConfigShapes(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		content string
		want    string
	}{
		{"dashy", "dashy", "pageInfo:\n  title: Family\nsections:\n  - name: Media\n    items:\n      - title: Jellyfin\n        url: http://jellyfin.local:8096\n", "Jellyfin"},
		{"homepage", "homepage", "Media:\n  - Jellyfin:\n      href: http://jellyfin.local:8096\n      description: Watch together\n", "Jellyfin"},
		{"homarr json", "homarr", `{"apps":[{"name":"Jellyfin","url":"http://jellyfin.local:8096"}]}`, "Jellyfin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preview, err := PreviewExternal(ExternalImport{Source: test.source, Content: test.content})
			if err != nil {
				t.Fatal(err)
			}
			if preview.Title == "" || len(preview.Apps) != 1 || preview.Apps[0].Name != test.want || preview.Apps[0].URL != "http://jellyfin.local:8096" {
				t.Fatalf("unexpected preview: %+v", preview)
			}
		})
	}
}

func TestPreviewExternalRejectsUnsupportedOrOversizedInput(t *testing.T) {
	tests := []ExternalImport{
		{Source: "unknown", Content: `{}`},
		{Source: "dashy", Content: strings.Repeat("x", MaxExternalImportSize+1)},
		{Source: "homepage", Content: "not: [valid"},
		{Source: "homarr", Content: `{"settings": {}}`},
	}
	for _, input := range tests {
		if _, err := PreviewExternal(input); err == nil {
			t.Fatalf("PreviewExternal(%q) error = nil", input.Source)
		}
	}
}

func TestImportExternalValidatesBeforeReplacingBoard(t *testing.T) {
	service, store := serviceForTest(t, testBoard(testApp(1, "https://existing.example.test", false)))
	if _, err := service.ImportExternal(context.Background(), ExternalImport{Source: "dashy", Content: `sections: []`}, 1, "Owner"); err == nil {
		t.Fatal("expected empty external import to fail")
	}
	if store.saveCount != 0 || len(service.Snapshot().Apps) != 1 {
		t.Fatal("rejected external import caused side effects")
	}

	receipt, err := service.ImportExternal(context.Background(), ExternalImport{Source: "homepage", Content: "Media:\n  - Jellyfin:\n      href: http://jellyfin.local:8096\n"}, 1, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.AfterVersion != 2 || len(service.Snapshot().Apps) != 2 || service.Snapshot().Apps[1].Name != "Jellyfin" {
		t.Fatalf("unexpected imported board: receipt=%+v board=%+v", receipt, service.Snapshot())
	}
	if _, err := service.ImportExternal(context.Background(), ExternalImport{Source: "homepage", Content: "Media:\n  - Plex:\n      href: http://plex.local:32400\n"}, 1, "Owner"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale import error = %v, want conflict", err)
	}
}

func TestExternalImportRejectsUnreloadableTitleBeforeEffects(t *testing.T) {
	for _, title := range []string{strings.Repeat("x", 61), "\x00bad"} {
		service, store := serviceForTest(t, testBoard())
		before := service.Snapshot()
		input := ExternalImport{Source: "dashy", Content: "pageInfo:\n  title: " + title + "\nsections:\n  - items:\n      - title: Media\n        url: https://media.example\n"}
		if _, err := service.ImportExternal(context.Background(), input, before.Version, "Owner"); err == nil {
			t.Fatal("invalid title accepted")
		}
		if store.saveCount != 0 || service.Snapshot().Version != before.Version || len(service.Snapshot().Apps) != 0 {
			t.Fatal("invalid import had effects")
		}
	}
}
