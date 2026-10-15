package servertest

import "testing"

func MetadataStoreDetachesMutableRecords(t *testing.T, set func(map[string]string, map[string]string) error, project func() map[string]string) {
	t.Helper()
	providerIDs := map[string]string{"tmdb": "329865"}
	if err := set(providerIDs, map[string]string{"tmdb": "1"}); err != nil {
		t.Fatal(err)
	}
	providerIDs["tmdb"] = "changed"
	first := project()
	if first["tmdb"] != "329865" {
		t.Fatalf("caller mutation reached metadata store: %#v", first)
	}
	first["tmdb"] = "changed again"
	second := project()
	if second["tmdb"] != "329865" {
		t.Fatalf("projection mutation reached metadata store: %#v", second)
	}
}
