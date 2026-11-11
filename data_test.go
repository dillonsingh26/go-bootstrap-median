package main

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

func TestReadValues(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     []float64
	}{
		{"one column", "value\n1.5\n2\n-3\n", []float64{1.5, 2, -3}},
		{"value is not the first column", "id,value\n1,10\n2,20\n", []float64{10, 20}},
		{"spreadsheet byte-order mark and spaces", "\ufeffid, value\n1, 10\n2, 20\n", []float64{10, 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), "data.csv", tt.contents)
			got, err := readValues(path)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("readValues = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadValuesExplainsBadFiles(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{"empty file", "", "is empty"},
		{"header only", "value\n", "has a header row but no data"},
		{"no value column", "x\n1\n2\n", `no column named "value" (columns found: x)`},
		{"text instead of a number", "value\n1\nabc\n", `line 3, column "value": "abc" is not a number`},
		{"blank entry", "id,value\n1,4\n2,\n", `line 3, column "value": "" is not a number`},
		{"missing value", "value\n1\nNaN\n", `"NaN" is not a number`},
		{"infinite value", "value\n1\nInf\n", `"Inf" is not a number`},
		{"too few values", "value\n5\n", "at least 2 are needed"},
		{"row with too few fields", "id,value\n1,2\n3\n", "wrong number of fields"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), "data.csv", tt.contents)
			_, err := readValues(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("readValues error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestReadValuesOfMissingFile(t *testing.T) {
	if _, err := readValues("does-not-exist.csv"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("readValues error = %v, want a file-not-found error", err)
	}
}
