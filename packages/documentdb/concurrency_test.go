package documentdb

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

type revisionDocument struct {
	Revision int `json:"revision"`
}

func TestConcurrentExportsObserveOnlyAtomicBatchRevisions(t *testing.T) {
	store, err := Open(t.TempDir(), true, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := saveRevision(store, 0); err != nil {
		t.Fatal(err)
	}

	const readers, reads, writes = 8, 80, 40
	start := make(chan struct{})
	failures := make(chan error, readers+1)
	var group sync.WaitGroup
	group.Add(readers + 1)
	go func() {
		defer group.Done()
		<-start
		if err := writeRevisions(store, writes); err != nil {
			failures <- err
		}
	}()
	for range readers {
		go func() {
			defer group.Done()
			<-start
			if err := readRevisions(store, reads); err != nil {
				failures <- err
			}
		}()
	}
	close(start)
	group.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var final revisionDocument
	if found, err := store.LoadJSON("settings.json", &final); err != nil || !found || final.Revision != writes {
		t.Fatalf("final revision = %#v, found=%t, error=%v", final, found, err)
	}
}

func saveRevision(store *Store, revision int) error {
	value := revisionDocument{Revision: revision}
	return store.SaveJSONBatch(map[string]any{"settings.json": value, "profiles.json": value})
}

func writeRevisions(store *Store, writes int) error {
	for revision := 1; revision <= writes; revision++ {
		if err := saveRevision(store, revision); err != nil {
			return err
		}
	}
	return nil
}

func readRevisions(store *Store, reads int) error {
	for range reads {
		exported, err := store.Export()
		if err != nil {
			return err
		}
		if err := matchingRevision(exported); err != nil {
			return err
		}
	}
	return nil
}

func matchingRevision(exported map[string][]byte) error {
	var settings, profiles revisionDocument
	if err := json.Unmarshal(exported["settings.json"], &settings); err != nil {
		return err
	}
	if err := json.Unmarshal(exported["profiles.json"], &profiles); err != nil {
		return err
	}
	if settings.Revision != profiles.Revision {
		return fmt.Errorf("mixed batch revisions: settings=%d profiles=%d", settings.Revision, profiles.Revision)
	}
	return nil
}
