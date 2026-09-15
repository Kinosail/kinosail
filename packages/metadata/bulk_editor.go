package metadata

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail/packages/library"
)

const maxBulkItems = 100

var (
	// ErrBulkSelection reports an invalid bulk metadata item selection.
	ErrBulkSelection = errors.New("metadata selection is invalid")
	// ErrBulkFields reports an invalid bulk metadata patch.
	ErrBulkFields = errors.New("metadata fields are invalid")
	errBulkSave   = errors.New("metadata could not be saved")
)

// BulkPatch contains the owner-controlled fields that a bulk edit can replace.
type BulkPatch struct {
	Title   *string `json:"title"`
	Year    *string `json:"year"`
	Plot    *string `json:"plot"`
	Rating  *string `json:"rating"`
	Tagline *string `json:"tagline"`
	Genres  *string `json:"genres"`
}

// BulkIndex supplies the library operations required by a bulk metadata edit.
type BulkIndex interface {
	Find(string) (library.Item, bool)
	Refresh(context.Context) error
	Snapshot() ([]library.Item, error)
}

// BulkEditor owns Player's canonical bulk metadata operation.
type BulkEditor struct {
	index    BulkIndex
	recordMu sync.Locker
	records  *map[string]Record
	save     func(map[string]Record) error
	page     BulkPage
	render   BulkRender
	fail     BulkFailure
}

// NewBulkEditor connects storage and rendering adapters to Player's canonical operation.
func NewBulkEditor(index BulkIndex, recordMu sync.Locker, records *map[string]Record, save func(map[string]Record) error, render BulkRender, fail BulkFailure) *BulkEditor {
	return NewBulkEditorFor(index, recordMu, records, save, PlayerBulkPage(), render, fail)
}

// NewBulkEditorFor connects a derivative product profile to Player's canonical operation.
func NewBulkEditorFor(index BulkIndex, recordMu sync.Locker, records *map[string]Record, save func(map[string]Record) error, page BulkPage, render BulkRender, fail BulkFailure) *BulkEditor {
	return &BulkEditor{index: index, recordMu: recordMu, records: records, save: save, page: page, render: render, fail: fail}
}

// Apply validates and persists one owner-authenticated bulk metadata edit.
func (editor *BulkEditor) Apply(ctx context.Context, ids []string, patch BulkPatch) (map[string]Record, error) {
	ids, err := normalizeBulkSelection(append([]string(nil), ids...))
	if err != nil {
		return nil, err
	}
	patch, err = patch.normalized()
	if err != nil {
		return nil, err
	}
	items := make([]library.Item, len(ids))
	for position, id := range ids {
		item, found := editor.index.Find(id)
		if !found {
			return nil, ErrBulkSelection
		}
		items[position] = item
	}
	updates := editor.snapshotRecords(items)
	for _, item := range items {
		record := recordWithLibraryFallback(item, updates[item.ID])
		patch.apply(&record)
		record.Owner = true
		if !Valid(record) {
			return nil, ErrBulkFields
		}
		updates[item.ID] = record
	}
	if err := editor.save(updates); err != nil {
		return nil, errBulkSave
	}
	if err := editor.index.Refresh(ctx); err != nil {
		return nil, errBulkSave
	}
	return updates, nil
}

func (editor *BulkEditor) snapshotRecords(items []library.Item) map[string]Record {
	editor.recordMu.Lock()
	defer editor.recordMu.Unlock()
	records := make(map[string]Record, len(items))
	for _, item := range items {
		records[item.ID] = (*editor.records)[item.ID]
	}
	return records
}

func normalizeBulkSelection(ids []string) ([]string, error) {
	if len(ids) == 0 || len(ids) > maxBulkItems {
		return nil, ErrBulkSelection
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, found := seen[id]; found || !validBulkID(id) {
			return nil, ErrBulkSelection
		}
		seen[id] = struct{}{}
	}
	return ids, nil
}

func validBulkID(id string) bool {
	if id == "" || id != strings.TrimSpace(id) || len(id) != 16 {
		return false
	}
	for _, character := range id {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func (patch BulkPatch) normalized() (BulkPatch, error) {
	patch.Title = trimBulkValue(patch.Title)
	patch.Year = trimBulkValue(patch.Year)
	patch.Plot = trimBulkValue(patch.Plot)
	patch.Rating = trimBulkValue(patch.Rating)
	patch.Tagline = trimBulkValue(patch.Tagline)
	patch.Genres = trimBulkValue(patch.Genres)
	if patch.empty() {
		return BulkPatch{}, ErrBulkFields
	}
	if patch.Year != nil && *patch.Year != "" && (len(*patch.Year) != 4 || strings.Trim(*patch.Year, "0123456789") != "") {
		return BulkPatch{}, ErrBulkFields
	}
	probe := Record{Title: "valid"}
	patch.apply(&probe)
	if !Valid(probe) {
		return BulkPatch{}, ErrBulkFields
	}
	return patch, nil
}

func (patch BulkPatch) empty() bool {
	return patch.Title == nil && patch.Year == nil && patch.Plot == nil && patch.Rating == nil && patch.Tagline == nil && patch.Genres == nil
}

func trimBulkValue(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func (patch BulkPatch) apply(record *Record) {
	if patch.Title != nil {
		record.Title = *patch.Title
	}
	if patch.Year != nil {
		record.Year = *patch.Year
	}
	if patch.Plot != nil {
		record.Plot = *patch.Plot
	}
	if patch.Rating != nil {
		record.Rating = *patch.Rating
	}
	if patch.Tagline != nil {
		record.Tagline = *patch.Tagline
	}
	if patch.Genres != nil {
		record.Genres = *patch.Genres
	}
}

func recordWithLibraryFallback(item library.Item, record Record) Record {
	if record.Title == "" {
		record.Title = item.Title
	}
	if record.Year == "" {
		record.Year = item.Year
	}
	if record.Plot == "" {
		record.Plot = item.Plot
	}
	if record.Rating == "" {
		record.Rating = item.Rating
	}
	if record.Tagline == "" {
		record.Tagline = item.Tagline
	}
	if record.Genres == "" {
		record.Genres = item.Genres
	}
	if record.Collection == "" {
		record.Collection = item.Collection
	}
	return record
}
