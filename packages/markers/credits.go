package markers

import (
	"context"
	"errors"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const creditFrameWidth, creditFrameHeight = 160, 90

func (analyzer *Analyzer) visualCredits(ctx context.Context, item library.Item, duration float64) ([]Marker, error) {
	if !finite(duration) || duration <= 0 {
		return nil, errors.New("credit analysis duration is invalid")
	}
	window := min(duration*.2, 900)
	if window < 20 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	frames, err := analyzer.creditFrames(ctx, item, duration-window, window)
	if err != nil {
		return nil, err
	}
	frameSize := creditFrameWidth * creditFrameHeight
	if len(frames)%frameSize != 0 {
		return nil, errors.New("credit analysis returned malformed frames")
	}
	points := make([]float64, 0, len(frames)/frameSize)
	for index := 0; index < len(frames)/frameSize; index++ {
		if creditFrameLikely(frames[index*frameSize : (index+1)*frameSize]) {
			points = append(points, float64(index))
		}
	}
	return creditMarkers(points, duration-window, duration), nil
}

func (analyzer *Analyzer) creditFrames(ctx context.Context, item library.Item, start, length float64) ([]byte, error) {
	if analyzer.extractCredits != nil {
		return analyzer.extractCredits(ctx, item, start, length)
	}
	command := analysisCommand(ctx, analyzer.ffmpeg, "-ss", seconds(start), "-i", item.Path, "-t", seconds(length), "-map", "0:v:0", "-vf", "fps=1,scale=160:90:force_original_aspect_ratio=decrease,pad=160:90:(ow-iw)/2:(oh-ih)/2,format=gray", "-an", "-f", "rawvideo", "-pix_fmt", "gray", "-")
	return command.Output()
}

func creditFrameLikely(frame []byte) bool { //nolint:cyclop,gocognit // Spatial edge and background checks form one bounded frame classifier.
	if len(frame) != creditFrameWidth*creditFrameHeight {
		return false
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
	return backgroundRatio >= .4 && edgeRatio >= .015 && edgeRatio <= .3 && activeRows >= height/8 && activeColumns >= width/4
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func creditMarkers(points []float64, offset, duration float64) []Marker { //nolint:cyclop // Consecutive text-like tail runs form one marker-classification pass.
	markers := make([]Marker, 0)
	for start := 0; start < len(points); {
		end := start
		for end+1 < len(points) && points[end+1]-points[end] <= 2.5 {
			end++
		}
		if points[end]-points[start] >= 20 {
			markerEnd := offset + points[end] + 1
			if duration-markerEnd <= 5 {
				markerEnd = duration
			}
			markers = append(markers, Marker{Type: "credits", Label: "Credits", Start: offset + points[start], End: markerEnd, Source: "visual"})
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
