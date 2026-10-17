package mediaprobe

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
)

const (
	imageSubtitleFrameBytes  = 64 << 20
	imageSubtitleCanvasLimit = 8192
	imageSubtitleCueLimit    = 20000
)

var imageSubtitleFrameName = regexp.MustCompile(`^\d{12}\.png$`)

type imageCue struct {
	start, end int64
	path       string
	top        bool
	key        [32]byte
}

func imageSubtitleCues(directory string) ([]imageCue, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() && imageSubtitleFrameName.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	if len(names) > imageSubtitleFrameLimit {
		return nil, errImageSubtitleFrames
	}
	slices.Sort(names)
	builder := imageCueBuilder{directory: directory, last: -1}
	for _, name := range names {
		if err = builder.add(name); err != nil {
			return nil, err
		}
	}
	cues := builder.cues
	return slices.DeleteFunc(cues, func(cue imageCue) bool { return cue.end-cue.start < 40 }), nil
}

type imageCueBuilder struct {
	directory string
	cues      []imageCue
	open      bool
	last      int64
}

func (builder *imageCueBuilder) add(name string) error {
	at, _ := strconv.ParseInt(name[:12], 10, 64)
	if at < builder.last || at >= 1<<32 {
		return nil
	}
	builder.last = at
	frame, bounds, err := readImageSubtitleFrame(filepath.Join(builder.directory, name))
	if err != nil {
		return err
	}
	if bounds.Empty() {
		if builder.open {
			builder.cues[len(builder.cues)-1].end, builder.open = at, false
		}
		return nil
	}
	key := imageCueKey(frame, bounds)
	if builder.open && builder.cues[len(builder.cues)-1].key == key {
		return nil
	}
	if builder.open {
		builder.cues[len(builder.cues)-1].end = at
	}
	if len(builder.cues) == imageSubtitleCueLimit {
		return errImageSubtitleFrames
	}
	path := filepath.Join(builder.directory, "cue-"+strconv.Itoa(len(builder.cues))+".png")
	if err = writeImageCue(path, frame, bounds); err != nil {
		return err
	}
	builder.cues, builder.open = append(builder.cues, imageCue{start: at, end: at + 5000, path: path, top: bounds.Max.Y < frame.Bounds().Dy()/3, key: key}), true
	return nil
}

func readImageSubtitleFrame(path string) (*image.NRGBA, image.Rectangle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, imageSubtitleFrameBytes+1))
	if err != nil || len(data) > imageSubtitleFrameBytes {
		return nil, image.Rectangle{}, errImageSubtitleFrames
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > imageSubtitleCanvasLimit || config.Height > imageSubtitleCanvasLimit {
		return nil, image.Rectangle{}, errImageSubtitleFrames
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, image.Rectangle{}, errImageSubtitleFrames
	}
	frame, converted := decoded.(*image.NRGBA)
	if !converted {
		frame = image.NewNRGBA(decoded.Bounds())
		draw.Draw(frame, frame.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	}
	return frame, visibleBounds(frame), nil
}

func visibleBounds(frame *image.NRGBA) image.Rectangle {
	bounds := frame.Bounds()
	visible := image.Rectangle{Min: bounds.Max, Max: bounds.Min}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := frame.Pix[(y-bounds.Min.Y)*frame.Stride:]
		for x := 0; x < bounds.Dx(); x++ {
			if row[x*4+3] > 16 {
				visible.Min.X, visible.Min.Y = min(visible.Min.X, bounds.Min.X+x), min(visible.Min.Y, y)
				visible.Max.X, visible.Max.Y = max(visible.Max.X, bounds.Min.X+x+1), max(visible.Max.Y, y+1)
			}
		}
	}
	if visible.Min.X >= visible.Max.X {
		return image.Rectangle{}
	}
	return visible
}

func imageCueKey(frame *image.NRGBA, bounds image.Rectangle) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(bounds.String()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		start := frame.PixOffset(bounds.Min.X, y)
		_, _ = hash.Write(frame.Pix[start : start+bounds.Dx()*4])
	}
	var key [32]byte
	copy(key[:], hash.Sum(nil))
	return key
}

func writeImageCue(path string, frame *image.NRGBA, bounds image.Rectangle) error {
	output := binaryImageCue(frame, bounds, otsuThreshold(opaqueLuminance(frame, bounds)))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err = png.Encode(file, output); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func opaqueLuminance(frame *image.NRGBA, bounds image.Rectangle) [256]int {
	var histogram [256]int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if pixel := frame.NRGBAAt(x, y); pixel.A > 128 {
				histogram[luminance(pixel)]++
			}
		}
	}
	return histogram
}

func binaryImageCue(frame *image.NRGBA, bounds image.Rectangle, threshold int) *image.Gray {
	scale := 1
	if bounds.Dy() < 120 {
		scale = 2
	}
	const padding = 16
	output := image.NewGray(image.Rect(0, 0, bounds.Dx()*scale+padding*2, bounds.Dy()*scale+padding*2))
	for index := range output.Pix {
		output.Pix[index] = 255
	}
	for y := 0; y < bounds.Dy()*scale; y++ {
		for x := 0; x < bounds.Dx()*scale; x++ {
			if pixel := frame.NRGBAAt(bounds.Min.X+x/scale, bounds.Min.Y+y/scale); pixel.A > 128 && luminance(pixel) > threshold {
				output.SetGray(padding+x, padding+y, color.Gray{})
			}
		}
	}
	return output
}

func luminance(pixel color.NRGBA) int {
	return (299*int(pixel.R) + 587*int(pixel.G) + 114*int(pixel.B)) / 1000
}

func otsuThreshold(histogram [256]int) int {
	total, sum := 0, 0
	for value, count := range histogram {
		total, sum = total+count, sum+value*count
	}
	best, threshold, background, backgroundSum := 0.0, 127, 0, 0
	for value, count := range histogram {
		background, backgroundSum = background+count, backgroundSum+value*count
		foreground := total - background
		if background == 0 || foreground == 0 {
			continue
		}
		backgroundMean := float64(backgroundSum) / float64(background)
		foregroundMean := float64(sum-backgroundSum) / float64(foreground)
		if variance := float64(background) * float64(foreground) * (backgroundMean - foregroundMean) * (backgroundMean - foregroundMean); variance > best {
			best, threshold = variance, value
		}
	}
	return threshold
}
