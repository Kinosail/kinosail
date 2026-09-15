package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type block struct {
	start, end int
	covered    bool
}
type coverageProfile struct {
	files    map[string][]block
	modified time.Time
}

var coverageLine = regexp.MustCompile(`^([^\x00\r\n]+):([0-9]+)\.([0-9]+),([0-9]+)\.([0-9]+) ([0-9]+) ([0-9]+)$`)

func readCoverage(path string) (coverageProfile, error) {
	if err := validateFile(path, 64<<20); err != nil {
		return coverageProfile{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return coverageProfile{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return coverageProfile{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return coverageProfile{}, errors.New("coverage must be a regular file of at most 64 MiB")
	}
	limited := &io.LimitedReader{R: file, N: (64 << 20) + 1}
	files, err := parseCoverage(limited)
	if err == nil && limited.N == 0 {
		err = errors.New("coverage exceeds 64 MiB")
	}
	return coverageProfile{files: files, modified: info.ModTime()}, err
}

func parseCoverage(reader io.Reader) (map[string][]block, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 8192)
	if !scanner.Scan() {
		return nil, errors.New("missing coverage mode")
	}
	mode := scanner.Text()
	if mode != "mode: set" && mode != "mode: count" && mode != "mode: atomic" {
		return nil, errors.New("unknown coverage mode")
	}
	files := map[string][]block{}
	rows := 0
	for scanner.Scan() {
		rows++
		if rows > 1000000 {
			return nil, errors.New("too many coverage blocks")
		}
		name, value, err := parseBlock(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("coverage line %d: %w", rows+1, err)
		}
		files[name] = append(files[name], value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, errors.New("empty coverage profile")
	}
	return files, nil
}

func parseBlock(line string) (string, block, error) {
	fields := coverageLine.FindStringSubmatch(line)
	if fields == nil {
		return "", block{}, errors.New("malformed coverage block")
	}
	numbers := make([]uint64, 6)
	for i := range numbers {
		value, err := strconv.ParseUint(fields[i+2], 10, 63)
		if err != nil {
			return "", block{}, err
		}
		numbers[i] = value
	}
	if numbers[0] == 0 || numbers[1] == 0 || numbers[2] < numbers[0] || numbers[3] == 0 || numbers[0] > maxSourceSize || numbers[1] > maxSourceSize || numbers[2] > maxSourceSize || numbers[3] > maxSourceSize ||
		(numbers[2] == numbers[0] && numbers[3] < numbers[1]) || strings.TrimSpace(fields[1]) != fields[1] {
		return "", block{}, errors.New("invalid coverage coordinates")
	}
	return fields[1], block{int(numbers[0]), int(numbers[2]), numbers[5] > 0}, nil
}

// The established CRAP gate measures covered blocks, rather than weighting
// statements. Duplicate line spans are merged with logical OR, as before.
func coveredFraction(blocks []block, start, end int) float64 {
	seen := map[[2]int]bool{}
	for _, b := range blocks {
		if b.start >= start && b.start <= end {
			key := [2]int{b.start, b.end}
			seen[key] = seen[key] || b.covered
		}
	}
	if len(seen) == 0 {
		return 0
	}
	covered := 0
	for _, yes := range seen {
		if yes {
			covered++
		}
	}
	return float64(covered) / float64(len(seen))
}
