package server

import (
	"crypto/sha256"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/isobmff"
)

var (
	copiedAACMasterAttribute = regexp.MustCompile(`^([A-Z][A-Z0-9-]*)=("[^"\r\n]*"|[^,"\r\n]+)(,|$)`)
	copiedAACMasterInteger   = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)
	copiedAACMasterDecimal   = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,3})?$`)
	copiedAACMasterCodecs    = regexp.MustCompile(`^"[A-Za-z0-9]+([.,][A-Za-z0-9]+)*"$`)
)

type copiedAACMasterEntry struct {
	rendition  string
	attributes map[string]string
}

// Accept only the generated single-video grammar, including its older header.
func parseCopiedAACMaster(master []byte, policy string) (copiedAACMasterEntry, bool) {
	lines, valid := copiedAACMasterLines(master, policy)
	if !valid {
		return copiedAACMasterEntry{}, false
	}
	if lines[0] == "#KINOSAIL-BANDWIDTH:2" {
		lines = lines[1:]
	}
	if lines[0] != "#EXT-X-VERSION:7" {
		return copiedAACMasterEntry{}, false
	}
	lines = lines[1:]
	if lines[0] == "#EXT-X-INDEPENDENT-SEGMENTS" {
		lines = lines[1:]
	}
	if len(lines) != 3 || lines[2] != "" || !strings.HasPrefix(lines[0], "#EXT-X-STREAM-INF:") {
		return copiedAACMasterEntry{}, false
	}
	rendition, valid := copiedAACMasterRendition(lines[1])
	if !valid {
		return copiedAACMasterEntry{}, false
	}
	attributes, valid := copiedAACMasterAttributes(strings.TrimPrefix(lines[0], "#EXT-X-STREAM-INF:"))
	return copiedAACMasterEntry{rendition, attributes}, valid
}

func copiedAACMasterLines(master []byte, policy string) ([]string, bool) {
	if len(master) == 0 || len(master) > maximumCopiedHLSTimelineBytes || strings.ContainsAny(string(master), "\r\x00") {
		return nil, false
	}
	lines := strings.Split(string(master), "\n")
	if len(lines) < 6 || len(lines) > 8 {
		return nil, false
	}
	if lines[0] != "#EXTM3U" || lines[1] != "#KINOSAIL-TRANSCODER:"+policy {
		return nil, false
	}
	return lines[2:], true
}

func copiedAACMasterRendition(uri string) (string, bool) {
	rendition, suffix, found := strings.Cut(uri, "/")
	return rendition, found && suffix == "index.m3u8" && hlsFile(uri) && rendition != "audio"
}

func copiedAACMasterAttributes(line string) (map[string]string, bool) {
	if len(line) == 0 || len(line) > 1024 {
		return nil, false
	}
	attributes := make(map[string]string, 7)
	for line != "" {
		match := copiedAACMasterAttribute.FindStringSubmatch(line)
		if match == nil || len(attributes) >= 7 || !copiedAACMasterAttributeKnown(match[1]) || attributes[match[1]] != "" {
			return nil, false
		}
		attributes[match[1]] = match[2]
		line = line[len(match[0]):]
		if match[3] == "," && line == "" {
			return nil, false
		}
	}
	return attributes, copiedAACMasterValues(attributes)
}

func copiedAACMasterAttributeKnown(name string) bool {
	switch name {
	case "BANDWIDTH", "AVERAGE-BANDWIDTH", "CODECS", "RESOLUTION", "FRAME-RATE", "VIDEO-RANGE", "CLOSED-CAPTIONS":
		return true
	}
	return false
}

func copiedAACMasterValues(attributes map[string]string) bool {
	average, averageOK := copiedAACMasterNumber(attributes["AVERAGE-BANDWIDTH"])
	peak, peakOK := copiedAACMasterNumber(attributes["BANDWIDTH"])
	if !averageOK || !peakOK || average > peak {
		return false
	}
	if !copiedAACMasterCodecs.MatchString(attributes["CODECS"]) || attributes["CLOSED-CAPTIONS"] != "NONE" {
		return false
	}
	if !copiedAACMasterResolution(attributes["RESOLUTION"]) || !copiedAACMasterRange(attributes["VIDEO-RANGE"]) {
		return false
	}
	return copiedAACMasterFrameRate(attributes["FRAME-RATE"])
}

func copiedAACMasterNumber(value string) (int64, bool) {
	if !copiedAACMasterInteger.MatchString(value) {
		return 0, false
	}
	number, err := strconv.ParseInt(value, 10, 64)
	return number, err == nil && number > 0
}

func copiedAACMasterResolution(value string) bool {
	width, height, found := strings.Cut(value, "x")
	_, widthOK := copiedAACMasterNumber(width)
	_, heightOK := copiedAACMasterNumber(height)
	return found && widthOK && heightOK
}

func copiedAACMasterRange(value string) bool {
	return value == "SDR" || value == "HLG" || value == "PQ"
}

func copiedAACMasterFrameRate(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 20 || !copiedAACMasterDecimal.MatchString(value) {
		return false
	}
	rate, err := strconv.ParseFloat(value, 64)
	return err == nil && rate > 0 && !math.IsInf(rate, 0)
}

func (held *copiedAACGeneration) masterAllowed(master []byte) bool {
	entry, valid := parseCopiedAACMaster(master, held.policy)
	if !valid || entry.rendition != held.certificate.Rendition {
		return false
	}
	data, err := copiedHLSCacheFile(held.media, "init.mp4", isobmff.MaximumInitializationBytes)
	if err != nil || sha256.Sum256(data) != held.certificate.Initialization || held.ctx.Err() != nil {
		return false
	}
	initialization, err := isobmff.Parse(data)
	video, found := initialization.Video()
	if err != nil || !found {
		return false
	}
	return entry.attributes["CODECS"] == strconv.Quote(initialization.Codecs()) &&
		entry.attributes["RESOLUTION"] == strconv.Itoa(video.Width)+"x"+strconv.Itoa(video.Height) &&
		entry.attributes["VIDEO-RANGE"] == video.Range
}
