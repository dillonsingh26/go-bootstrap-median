package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
)

// readValues reads the "value" column of a CSV file (at least 2 values needed).
func readValues(path string) ([]float64, error) {
	columns, err := readColumns(path, "value")
	if err != nil {
		return nil, err
	}
	values := columns[0]
	if len(values) < 2 {
		return nil, fmt.Errorf("%s has %d value; at least 2 are needed", path, len(values))
	}
	return values, nil
}

// readColumns reads named number columns from a CSV file, reporting the line of any bad entry.
func readColumns(path string, names ...string) ([][]float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s is empty", path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	positions, err := columnPositions(header, names)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	columns := make([][]float64, len(names))
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		for i, position := range positions {
			value, err := parseNumber(record[position])
			if err != nil {
				return nil, fmt.Errorf("%s line %d, column %q: %w", path, line, names[i], err)
			}
			columns[i] = append(columns[i], value)
		}
	}
	if len(columns[0]) == 0 {
		return nil, fmt.Errorf("%s has a header row but no data", path)
	}
	return columns, nil
}

// columnPositions finds each named column, ignoring spaces and Excel's hidden byte-order mark.
func columnPositions(header, names []string) ([]int, error) {
	for i, name := range header {
		header[i] = strings.TrimSpace(strings.TrimPrefix(name, "\ufeff"))
	}
	positions := make([]int, len(names))
	for i, name := range names {
		positions[i] = slices.Index(header, name)
		if positions[i] < 0 {
			return nil, fmt.Errorf("no column named %q (columns found: %s)",
				name, strings.Join(header, ", "))
		}
	}
	return positions, nil
}

// parseNumber converts one CSV entry to a finite number.
func parseNumber(text string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%q is not a number", text)
	}
	return value, nil
}
