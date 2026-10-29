package metadata

import (
	"encoding/json"
	"sync"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func NewBulkEditor(index BulkIndex, recordMu sync.Locker, records *map[string]Record, save func(map[string]Record) error, render BulkRender, fail BulkFailure) *BulkEditor {
	return NewBulkEditorFor(index, recordMu, records, save, PlayerBulkPage(), render, fail)
}

func saveJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return privatefile.WriteCache(path, data)
}
