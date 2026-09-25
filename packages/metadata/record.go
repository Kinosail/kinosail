// Package metadata owns Kinosail media metadata providers and validated records.
package metadata

import (
	"context"
	"errors"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/library"
)

// Record contains persisted metadata for one library item.
type Record struct {
	Title, Year, Plot, Rating, Tagline, Genres, Artwork, Backdrop, Collection string
	ShowTitle, ShowYear, ShowPlot, ShowArtwork, ShowBackdrop                  string
	Cast, ShowCast                                                            []library.Person
	Owner, CastFetched, BackdropChecked                                       bool
	ProviderIDs                                                               map[string]string `json:"providerIds,omitempty"`
	ShowProviderIDs                                                           map[string]string `json:"showProviderIds,omitempty"`
}

// Image describes one provider asset that must be cached with a resolved record.
type Image struct {
	Source, Target string
	Show           bool
	Person         bool
	Backdrop       bool
}

// Result contains one resolved provider record and its pending provider assets.
type Result struct {
	Record Record
	Images []Image
}

// Year returns a four-digit year from a validated provider date.
func Year(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

// Valid reports whether a record is complete and within persisted bounds.
func Valid(record Record) bool { return record.Title != "" && Bounded(record) }

// Bounded reports whether every persisted field is safe to store.
func Bounded(record Record) bool {
	fields := []struct {
		value     string
		limit     int
		multiline bool
	}{
		{record.Title, 200, false},
		{record.Year, 4, false},
		{record.Plot, 5000, true},
		{record.Rating, 32, false},
		{record.Tagline, 300, false},
		{record.Genres, 500, false},
		{record.Collection, 200, false},
		{record.Artwork, 4096, false},
		{record.Backdrop, 4096, false},
		{record.ShowTitle, 200, false},
		{record.ShowYear, 4, false},
		{record.ShowPlot, 5000, true},
		{record.ShowArtwork, 4096, false},
		{record.ShowBackdrop, 4096, false},
	}
	for _, field := range fields {
		if len(field.value) > field.limit || hasInvalidRecordText(field.value, field.multiline) {
			return false
		}
	}
	return validRecordCast(record.Cast) && validRecordCast(record.ShowCast) && validMetadataYear(record.Year) && validMetadataYear(record.ShowYear) && validProviderIDs(record.ProviderIDs) && validProviderIDs(record.ShowProviderIDs)
}

func hasInvalidRecordText(value string, multiline bool) bool {
	return strings.IndexFunc(value, func(character rune) bool {
		return unicode.IsControl(character) && (!multiline || character != '\n' && character != '\r' && character != '\t')
	}) >= 0
}

// ApplyMissing fills only empty scanned library fields from a stored record.
func ApplyMissing(item *library.Item, record Record) {
	applyRecordCast(item, record)
	fillString(&item.Year, record.Year)
	fillString(&item.Plot, record.Plot)
	fillString(&item.Artwork, record.Artwork)
	fillString(&item.Backdrop, record.Backdrop)
	fillString(&item.ShowTitle, record.ShowTitle)
	fillString(&item.ShowYear, record.ShowYear)
	fillString(&item.ShowPlot, record.ShowPlot)
	fillString(&item.ShowArtwork, record.ShowArtwork)
	fillString(&item.ShowBackdrop, record.ShowBackdrop)
	if len(item.ShowProviderIDs) == 0 && len(record.ShowProviderIDs) > 0 {
		item.ShowProviderIDs = maps.Clone(record.ShowProviderIDs)
	}
}

// Apply overlays every populated owner-controlled field onto one item.
func Apply(item *library.Item, record Record) {
	applyRecordCast(item, record)
	setString(&item.Title, record.Title)
	setString(&item.Year, record.Year)
	setString(&item.Plot, record.Plot)
	setString(&item.Rating, record.Rating)
	setString(&item.Tagline, record.Tagline)
	setString(&item.Genres, record.Genres)
	setString(&item.Artwork, record.Artwork)
	setString(&item.Backdrop, record.Backdrop)
	item.Collection = record.Collection
	if len(record.ProviderIDs) > 0 {
		item.ProviderIDs = record.ProviderIDs
	}
	setString(&item.ShowTitle, record.ShowTitle)
	setString(&item.ShowYear, record.ShowYear)
	setString(&item.ShowPlot, record.ShowPlot)
	setString(&item.ShowArtwork, record.ShowArtwork)
	setString(&item.ShowBackdrop, record.ShowBackdrop)
	if len(record.ShowProviderIDs) > 0 {
		item.ShowProviderIDs = record.ShowProviderIDs
	}
}

func fillString(target *string, value string) {
	if *target == "" {
		*target = value
	}
}

func setString(target *string, value string) {
	if value != "" {
		*target = value
	}
}

// ApplyRecords overlays one cloned record set and resolves cached artwork paths.
func ApplyRecords(items []library.Item, records map[string]Record, currentArtwork func(string) string) []library.Item {
	for position := range items {
		record, found := records[items[position].ID]
		if !found {
			continue
		}
		record = Clone(record)
		record.Artwork = currentArtwork(record.Artwork)
		record.Backdrop = currentArtwork(record.Backdrop)
		record.ShowArtwork = currentArtwork(record.ShowArtwork)
		record.ShowBackdrop = currentArtwork(record.ShowBackdrop)
		for _, cast := range [][]library.Person{record.Cast, record.ShowCast} {
			for index := range cast {
				cast[index].Image = currentArtwork(cast[index].Image)
			}
		}
		if !record.Owner && RegularFile(strings.TrimSuffix(items[position].Path, filepath.Ext(items[position].Path))+".nfo") {
			ApplyMissing(&items[position], record)
			continue
		}
		Apply(&items[position], record)
	}
	return items
}

// RegularFile reports whether a metadata sidecar or cached asset is a regular file.
func RegularFile(path string) bool {
	info, err := os.Lstat(path) //nolint:gosec // Callers derive the path from configured metadata storage or scanned Library Content.
	return err == nil && info.Mode().IsRegular()
}

func boundedMetadata(record Record) bool { return Bounded(record) }

func validProviderIDs(values map[string]string) bool {
	if len(values) > 16 {
		return false
	}
	for name, value := range values {
		if name == "" || len(name) > 32 || value == "" || len(value) > 128 || strings.IndexFunc(name+value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			return false
		}
	}
	return true
}

// ValidateRecords validates one complete persisted metadata state before use.
func ValidateRecords(records map[string]Record) error {
	if len(records) > 100000 {
		return errors.New("too many persisted metadata records")
	}
	for id, record := range records {
		if !validMetadataID(id) || !Bounded(record) {
			return errors.New("persisted metadata is invalid")
		}
	}
	return nil
}

func validMetadataID(value string) bool {
	return value != "" && len(value) <= 128 && !strings.ContainsAny(value, `/\`) && strings.IndexFunc(value, unicode.IsControl) < 0
}

// ResolveTMDBEpisode applies the shared episode response and artwork contract.
func ResolveTMDBEpisode(ctx context.Context, item library.Item, show Record, enrich func(context.Context, library.Item, int, string, string, *Record, *string), artwork func(string) string) (Result, error) { //nolint:cyclop // One fail-closed guard keeps all cross-field episode invariants ahead of the provider call.
	id, err := strconv.Atoi(show.ShowProviderIDs["tmdb"])
	if ctx == nil || enrich == nil || artwork == nil || err != nil || id <= 0 || !validMetadataID(item.ID) || item.Season < 0 || item.Season > 100000 || item.Episode < 0 || item.Episode > 100000 || show.ShowTitle == "" || !Bounded(show) {
		return Result{}, errors.New("show metadata is invalid")
	}
	record := Record{CastFetched: show.CastFetched, BackdropChecked: show.BackdropChecked, ShowCast: append([]library.Person(nil), show.ShowCast...), ShowTitle: show.ShowTitle, ShowYear: show.ShowYear, ShowPlot: show.ShowPlot, ShowArtwork: show.ShowArtwork, ShowBackdrop: show.ShowBackdrop, ShowProviderIDs: maps.Clone(show.ShowProviderIDs), ProviderIDs: TVDBProviderID(item)}
	poster := ""
	enrich(ctx, item, id, show.ShowTitle, show.ShowYear, &record, &poster)
	if !ValidTMDBPath(poster) || !Valid(record) {
		return Result{}, errors.New("metadata provider returned invalid data")
	}
	result := Result{Record: record}
	if poster != "" {
		result.Record.Artwork = artwork(item.ID)
		if !Bounded(result.Record) {
			return Result{}, errors.New("metadata artwork path is invalid")
		}
		result.Images = []Image{{Source: poster, Target: result.Record.Artwork}}
	}
	return result, nil
}

// TVDBProviderID copies a bounded TVDB identity from one library item.
func TVDBProviderID(item library.Item) map[string]string {
	if value := strings.TrimSpace(item.ProviderIDs["tvdb"]); value != "" {
		return map[string]string{"tvdb": value}
	}
	return nil
}

// Clone returns a record with independent provider identity maps.
func Clone(record Record) Record {
	record.Cast = append([]library.Person(nil), record.Cast...)
	record.ShowCast = append([]library.Person(nil), record.ShowCast...)
	record.ProviderIDs = maps.Clone(record.ProviderIDs)
	record.ShowProviderIDs = maps.Clone(record.ShowProviderIDs)
	return record
}

// MergeRecords clones current records and applies cloned updates.
func MergeRecords(current, updates map[string]Record) map[string]Record {
	records := make(map[string]Record, len(current)+len(updates))
	for key, value := range current {
		records[key] = Clone(value)
	}
	for key, value := range updates {
		records[key] = Clone(value)
	}
	return records
}

// RecordFromForm parses the bounded owner-editable metadata fields.
func RecordFromForm(form url.Values, record Record) (Record, bool) {
	fields := []struct {
		name   string
		limit  int
		target *string
	}{
		{"title", 200, &record.Title},
		{"year", 4, &record.Year},
		{"plot", 5000, &record.Plot},
		{"rating", 32, &record.Rating},
		{"tagline", 300, &record.Tagline},
		{"genres", 500, &record.Genres},
	}
	for _, field := range fields {
		values, found := form[field.name]
		if !found {
			*field.target = ""
			continue
		}
		if len(values) != 1 || len(values[0]) > field.limit {
			return Record{}, false
		}
		*field.target = strings.TrimSpace(values[0])
	}
	record.Owner = true
	return record, Valid(record)
}
