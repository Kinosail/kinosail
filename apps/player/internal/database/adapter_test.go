package database

import (
	"context"
	"reflect"
	"testing"
)

func TestPlayerDatabaseAdapterUsesConfiguredDocuments(t *testing.T) {
	original := Documents
	t.Cleanup(func() { Documents = original })
	Documents = []string{"custom.json"}
	if configured := config(); !reflect.DeepEqual(configured.Documents, Documents) {
		t.Fatalf("documents = %#v", configured.Documents)
	}
	store, err := Open(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenContext(context.Background(), t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
