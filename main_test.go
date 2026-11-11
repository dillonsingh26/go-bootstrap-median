package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes contents to dir/name and returns the path.
func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runProgram runs the program and returns its exit code and printed output.
func runProgram(args ...string) (code int, stdout, stderr string) {
	var out, errs bytes.Buffer
	code = run(args, &out, &errs)
	return code, out.String(), errs.String()
}

func TestParseFlagsRejectsUnusableOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"neither mode", []string{"-B", "100"}, "give either -file or -study"},
		{"both modes", []string{"-file", "a.csv", "-study", "data"}, "not both"},
		{"too few resamples", []string{"-file", "a.csv", "-B", "1"}, "-B must be at least 2"},
		{"resamples not a number", []string{"-file", "a.csv", "-B", "ten"}, `invalid value "ten"`},
		{"unknown option", []string{"-files", "a.csv"}, "flag provided but not defined: -files"},
		{"stray argument", []string{"-file", "a.csv", "extra"}, `unexpected argument "extra"`},
		{"no workers", []string{"-file", "a.csv", "-workers", "0"}, "-workers must be at least 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFlags(tt.args, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseFlags(%q) error = %v, want one containing %q", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestParseFlagsAcceptsUsableOptions(t *testing.T) {
	cfg, err := parseFlags([]string{"-study", "data/study", "-B", "100", "-seed", "7"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.studyDir != "data/study" || cfg.resamples != 100 || cfg.seed != 7 {
		t.Errorf("parseFlags gave %+v", cfg)
	}
}

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{"no options shows usage", nil, exitUsageError, "Usage:"},
		{"help shows usage", []string{"-h"}, exitOK, "Usage:"},
		{"bad option explains itself", []string{"-B", "1", "-file", "x.csv"}, exitUsageError, "Run bootmedian -h"},
		{"missing file fails", []string{"-file", "does-not-exist.csv"}, exitFailure, "no such file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := runProgram(tt.args...)
			if code != tt.wantCode || !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("exit code %d, want %d; stderr should contain %q but was:\n%s",
					code, tt.wantCode, tt.wantStderr, stderr)
			}
		})
	}
}

func TestRunFileReportsResultsAndLogs(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFile(t, dir, "data.csv", "value\n3\n5\n7\n9\n20\n")
	logPath := filepath.Join(dir, "run.log")

	code, stdout, stderr := runProgram("-file", dataPath, "-B", "200", "-log", logPath)
	if code != exitOK {
		t.Fatalf("exit code %d, stderr:\n%s", code, stderr)
	}
	for _, want := range []string{"median:              7.000", "SE of the median:", "95% percentile CI:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
	logText, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logText), "bootstrap finished") {
		t.Errorf("log file is missing the finish message:\n%s", logText)
	}
}

func TestRunWritesProfiles(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFile(t, dir, "data.csv", "value\n1\n2\n3\n4\n")
	cpuPath := filepath.Join(dir, "cpu.prof")
	memPath := filepath.Join(dir, "mem.prof")

	code, _, stderr := runProgram("-file", dataPath, "-B", "50", "-log", filepath.Join(dir, "run.log"),
		"-cpuprofile", cpuPath, "-memprofile", memPath)
	if code != exitOK {
		t.Fatalf("exit code %d, stderr:\n%s", code, stderr)
	}
	for _, path := range []string{cpuPath, memPath} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Errorf("profile %s was not written", path)
		}
	}
}
