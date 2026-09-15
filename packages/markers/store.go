package markers

import (
	"errors"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// ErrPersistence reports a failed durable marker change without exposing storage details.
var ErrPersistence = errors.New("playback segment results could not be saved")

func (analyzer *Analyzer) storeLocked(item library.Item, markers []Marker, suppressed []string, version int) error {
	next := analyzer.records
	if analyzer.file != "" {
		next = maps.Clone(next)
	}
	next[item.ID] = Record{Revision: mediaRevision(item), DetectorVersion: version, Markers: markers, Suppressed: suppressed}
	if analyzer.file != "" {
		if err := analyzer.persist(analyzer.file, next); err != nil {
			analyzer.err = ErrPersistence
			return ErrPersistence
		}
	}
	analyzer.records = next
	return nil
}

func (analyzer *Analyzer) replaceDetected(item library.Item, markerTypes []string, replacements []Marker) {
	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()
	record := analyzer.records[item.ID]
	markers := make([]Marker, 0, len(record.Markers)+len(replacements))
	suppressed := record.suppressed(item)
	replaced := make(map[string]bool, len(markerTypes))
	for _, markerType := range markerTypes {
		replaced[markerType] = true
	}
	if record.Revision == mediaRevision(item) {
		for _, marker := range record.Markers {
			if !replaced[marker.Type] || marker.Source == "manual" {
				markers = append(markers, marker)
			}
		}
	}
	for _, replacement := range replacements {
		if !slices.Contains(suppressed, replacement.Type) && !hasMarkerSource(markers, replacement.Type, "manual") {
			markers = append(markers, replacement)
		}
	}
	_ = analyzer.storeLocked(item, markers, suppressed, DetectorVersion)
}

func hasMarkerSource(markers []Marker, markerType, source string) bool {
	for _, marker := range markers {
		if marker.Type == markerType && marker.Source == source {
			return true
		}
	}
	return false
}

func (analyzer *Analyzer) Markers(item library.Item, chapters []Marker) []Marker {
	analyzer.mu.RLock()
	record, found := analyzer.records[item.ID]
	analyzer.mu.RUnlock()
	if !found || record.Revision != mediaRevision(item) {
		return chapters
	}
	suppressed := record.suppressed(item)
	manual, generated := make([]Marker, 0), make([]Marker, 0)
	for _, marker := range record.Markers {
		if slices.Contains(suppressed, marker.Type) {
			continue
		}
		if marker.Source == "manual" {
			manual = append(manual, marker)
		} else if record.DetectorVersion == DetectorVersion {
			generated = append(generated, marker)
		}
	}
	visibleChapters := make([]Marker, 0, len(chapters))
	for _, marker := range chapters {
		if !slices.Contains(suppressed, marker.Type) {
			visibleChapters = append(visibleChapters, marker)
		}
	}
	return mergePlaybackMarkers(mergePlaybackMarkers(manual, visibleChapters), generated)
}

func (analyzer *Analyzer) SetManual(item library.Item, markerType string, start, end, duration float64) error {
	markerType, err := normalizeManualMarker(markerType, start, end, duration)
	if err != nil {
		return err
	}
	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()
	record := analyzer.records[item.ID]
	markers := make([]Marker, 0, len(record.Markers)+1)
	if record.Revision == mediaRevision(item) {
		for _, marker := range record.Markers {
			if marker.Type != markerType {
				markers = append(markers, marker)
			}
		}
	}
	markers = append(markers, Marker{Type: markerType, Label: strings.ToUpper(markerType[:1]) + markerType[1:], Start: start, End: end, Source: "manual"})
	suppressed := slices.DeleteFunc(record.suppressed(item), func(value string) bool { return value == markerType })
	return analyzer.storeLocked(item, markers, suppressed, record.analysisVersion(item))
}

// Suppress removes one marker type and prevents automatic replacement for this revision.
func (analyzer *Analyzer) Suppress(item library.Item, markerType string) error {
	normalized, err := NormalizeAutoSkip([]string{markerType})
	if err != nil || len(normalized) != 1 {
		return errors.New("playback marker type is invalid")
	}
	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()
	record := analyzer.records[item.ID]
	visible := make([]Marker, 0, len(record.Markers))
	if record.Revision == mediaRevision(item) {
		for _, marker := range record.Markers {
			if marker.Type != normalized[0] {
				visible = append(visible, marker)
			}
		}
	}
	suppressed, _ := NormalizeAutoSkip(append(record.suppressed(item), normalized[0]))
	return analyzer.storeLocked(item, visible, suppressed, record.analysisVersion(item))
}

func (record Record) analysisVersion(item library.Item) int {
	if record.Revision == mediaRevision(item) {
		return record.DetectorVersion
	}
	return 0
}

func normalizeManualMarker(markerType string, start, end, duration float64) (string, error) {
	normalized, err := NormalizeAutoSkip([]string{markerType})
	if err != nil || len(normalized) != 1 || !finite(start) || !finite(end) || start < 0 || end <= start || duration > 0 && end > duration {
		return "", errors.New("playback marker is invalid")
	}
	return normalized[0], nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func (record Record) suppressed(item library.Item) []string {
	if record.Revision != mediaRevision(item) || len(record.Suppressed) > len(markerTypes) {
		return nil
	}
	values, err := NormalizeAutoSkip(record.Suppressed)
	if err != nil {
		return nil
	}
	return values
}

func (analyzer *Analyzer) setStatus(state string, err error) {
	analyzer.mu.Lock()
	analyzer.state, analyzer.err = state, err
	analyzer.mu.Unlock()
}

func (analyzer *Analyzer) Status() (string, int, string) {
	analyzer.mu.RLock()
	defer analyzer.mu.RUnlock()
	state, message := analyzer.state, ""
	if state != "running" && len(analyzer.jobs) != 0 {
		state = "queued"
	}
	if analyzer.err != nil {
		message = analyzer.err.Error()
	}
	items := 0
	for _, record := range analyzer.records {
		if record.DetectorVersion == DetectorVersion {
			items++
		}
	}
	return state, items, message
}

func mediaRevision(item library.Item) string {
	return strconv.FormatInt(item.Size, 10) + ":" + strconv.FormatInt(item.Added.UnixNano(), 10)
}
func seconds(value float64) string { return strconv.FormatFloat(value, 'f', 3, 64) }
