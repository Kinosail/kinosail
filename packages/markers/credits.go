package markers

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

const (
	creditFrameWidth, creditFrameHeight = 160, 90
	creditRollMinimumSeconds            = 30.0
	creditRollMotionShare               = .35
)

type creditFile struct {
	Revision string   `json:"revision"`
	Version  int      `json:"version"`
	Markers  []Marker `json:"markers"`
}

type creditProfile struct {
	likely bool
	rows   []int
}

type creditPoint struct {
	second float64
	moving bool
}

func (analyzer *Analyzer) visualCredits(ctx context.Context, item library.Item, duration float64) ([]Marker, error) {
	if !finite(duration) || duration <= 0 {
		return nil, errors.New("credit analysis duration is invalid")
	}
	window := min(duration*.2, 900)
	if window < 20 {
		return nil, nil
	}
	path := filepath.Join(analyzer.cache, item.ID+"-credits.json")
	var cached creditFile
	if analyzer.cache != "" && readCache(path, &cached) && cached.Revision == mediaRevision(item) && cached.Version == DetectorVersion && validCachedCredits(cached.Markers) {
		return cached.Markers, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var frames []byte
	if err := analyzer.slot(ctx, func() (err error) {
		frames, err = analyzer.creditFrames(ctx, item, duration-window, window)
		return err
	}); err != nil {
		return nil, err
	}
	frameSize := creditFrameWidth * creditFrameHeight
	if len(frames)%frameSize != 0 {
		return nil, errors.New("credit analysis returned malformed frames")
	}
	points := make([]creditPoint, 0, len(frames)/frameSize)
	var previous creditProfile
	for index := 0; index < len(frames)/frameSize; index++ {
		profile := creditFrameProfile(frames[index*frameSize : (index+1)*frameSize])
		if profile.likely {
			points = append(points, creditPoint{float64(index), previous.likely && creditTextMoves(previous.rows, profile.rows)})
		}
		previous = profile
	}
	markers := creditMarkers(points, duration-window, duration)
	if analyzer.cache != "" {
		_ = saveJSON(path, creditFile{Revision: mediaRevision(item), Version: DetectorVersion, Markers: markers})
	}
	return markers, nil
}

func validCachedCredits(markers []Marker) bool {
	if len(markers) > 64 {
		return false
	}
	for _, marker := range markers {
		if marker.Type != "credits" || !validPersistedMarker(marker) {
			return false
		}
	}
	return true
}

func (analyzer *Analyzer) creditFrames(ctx context.Context, item library.Item, start, length float64) ([]byte, error) {
	if analyzer.extractCredits != nil {
		return analyzer.extractCredits(ctx, item, start, length)
	}
	command := analysisCommand(ctx, analyzer.ffmpeg, analyzer.threads, "-ss", seconds(start), "-i", item.Path, "-t", seconds(length), "-map", "0:v:0", "-vf", "fps=1,scale=160:90:force_original_aspect_ratio=decrease,pad=160:90:(ow-iw)/2:(oh-ih)/2,format=gray", "-an", "-f", "rawvideo", "-pix_fmt", "gray", "-")
	return workload.BackgroundOutput(command)
}

func creditFrameLikely(frame []byte) bool { return creditFrameProfile(frame).likely }

func creditFrameProfile(frame []byte) creditProfile {
	if len(frame) != creditFrameWidth*creditFrameHeight {
		return creditProfile{}
	}
	width, height := creditFrameWidth, creditFrameHeight
	dark, bright, edges := 0, 0, 0
	rowEdges, columnEdges := make([]int, height), make([]int, width)
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			value := frame[y*width+x]
			if value < 48 {
				dark++
			} else if value > 207 {
				bright++
			}
			gradient := absInt(int(value)-int(frame[y*width+x-1])) + absInt(int(value)-int(frame[(y-1)*width+x]))
			if gradient >= 80 {
				edges++
				rowEdges[y]++
				columnEdges[x]++
			}
		}
	}
	interior := (width - 2) * (height - 2)
	activeRows, activeColumns := 0, 0
	for _, count := range rowEdges {
		if count >= max(3, width/40) {
			activeRows++
		}
	}
	for _, count := range columnEdges {
		if count >= max(2, height/45) {
			activeColumns++
		}
	}
	edgeRatio := float64(edges) / float64(interior)
	backgroundRatio := float64(max(dark, bright)) / float64(interior)
	likely := backgroundRatio >= .4 && edgeRatio >= .015 && edgeRatio <= .3 && activeRows >= height/8 && activeColumns >= width/4
	return creditProfile{likely: likely, rows: rowEdges}
}

func creditTextMoves(previous, current []int) bool {
	if len(previous) != len(current) || len(previous) == 0 {
		return false
	}
	static, best := rowProfileSimilarity(previous, current, 0), 0.0
	for shift := 2; shift <= len(previous)/2; shift++ {
		best = max(best, rowProfileSimilarity(previous, current, shift))
	}
	scrolls := best >= .6 && best >= static+.15
	changed := static < .5 && best < .5
	return scrolls || changed
}

func rowProfileSimilarity(previous, current []int, shift int) float64 {
	shared, previousTotal, currentTotal := 0, 0, 0
	for row := shift; row < len(previous); row++ {
		before, after := previous[row], current[row-shift]
		shared += min(before, after)
		previousTotal += before
		currentTotal += after
	}
	total := max(previousTotal, currentTotal)
	if total == 0 {
		return 0
	}
	return float64(shared) / float64(total)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func creditMarkers(points []creditPoint, offset, duration float64) []Marker {
	markers := make([]Marker, 0)
	for start := 0; start < len(points); {
		end, moving := start, 0
		for end+1 < len(points) && points[end+1].second-points[end].second <= 2.5 {
			end++
			if points[end].moving {
				moving++
			}
		}
		if points[end].second-points[start].second >= 20 {
			markerEnd := offset + points[end].second + 1
			if duration-markerEnd <= 5 {
				markerEnd = duration
			}
			source := "visual"
			if markerEnd == duration && markerEnd-(offset+points[start].second) >= creditRollMinimumSeconds && float64(moving) >= creditRollMotionShare*float64(end-start) {
				source = "text"
			}
			markers = append(markers, Marker{Type: "credits", Label: "Credits", Start: offset + points[start].second, End: markerEnd, Source: source})
		}
		start = end + 1
	}
	anchor := len(markers) - 1
	for anchor >= 0 && duration-markers[anchor].End > 90 {
		anchor--
	}
	if anchor < 0 {
		return nil
	}
	for anchor > 0 && markers[anchor].Start-markers[anchor-1].End <= 180 {
		anchor--
	}
	return markers[anchor:]
}
