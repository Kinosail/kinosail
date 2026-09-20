package main

import (
	"errors"
	"go/token"
	"path/filepath"
)

func halsteadPackage(paths []string) (map[string]metricReport, error) {
	if len(paths) == 0 || len(paths) > 10000 {
		return nil, errors.New("invalid source count")
	}
	requested := map[string]string{}
	directory := ""
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if directory != "" && filepath.Dir(absolute) != directory {
			return nil, errors.New("sources must share one package directory")
		}
		if _, exists := requested[absolute]; exists {
			return nil, errors.New("duplicate source")
		}
		if _, err = parseSource(token.NewFileSet(), absolute); err != nil {
			return nil, err
		}
		directory = filepath.Dir(absolute)
		requested[absolute] = path
	}
	set, files, info, err := typedPackage(paths[0])
	if err != nil {
		return nil, err
	}
	reports := map[string]metricReport{}
	for _, file := range files {
		if original, ok := requested[set.Position(file.Pos()).Filename]; ok {
			reports[original] = measureFunctions(set, file, info)
		}
	}
	if len(reports) != len(paths) {
		return nil, errors.New("source is not in the selected package")
	}
	return reports, nil
}
