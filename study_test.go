package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// smallStudyFile returns a tiny study file: 2 iterations of samples of size 5 and 9.
func smallStudyFile() string {
	var text strings.Builder
	text.WriteString("iteration,n,value\n")
	for iteration := 1; iteration <= 2; iteration++ {
		for _, n := range []int{5, 9} {
			for i := range n {
				fmt.Fprintf(&text, "%d,%d,%d\n", iteration, n, 90+i*iteration)
			}
		}
	}
	return text.String()
}

func TestReadStudySamplesGroupsRows(t *testing.T) {
	path := writeFile(t, t.TempDir(), "normal.csv", smallStudyFile())
	samples, err := readStudySamples(path)
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int
	for _, sample := range samples {
		sizes = append(sizes, len(sample.values))
		if len(sample.values) != sample.n {
			t.Errorf("sample has %d values but n = %d", len(sample.values), sample.n)
		}
	}
	if want := []int{5, 9, 5, 9}; !slices.Equal(sizes, want) {
		t.Errorf("sample sizes = %v, want %v", sizes, want)
	}
}

func TestReadStudySamplesChecksSampleSize(t *testing.T) {
	path := writeFile(t, t.TempDir(), "normal.csv", "iteration,n,value\n1,3,10\n1,3,11\n")
	_, err := readStudySamples(path)
	if err == nil || !strings.Contains(err.Error(), "has 2 values, but its n column says 3") {
		t.Errorf("readStudySamples error = %v, want a sample size mismatch", err)
	}
}

func TestRunStudyReportsEveryShapeAndSize(t *testing.T) {
	dir := t.TempDir()
	for _, shape := range studyShapes {
		writeFile(t, dir, shape+".csv", smallStudyFile())
	}
	code, stdout, stderr := runProgram("-study", dir, "-B", "50", "-log", dir+"/run.log")
	if code != exitOK {
		t.Fatalf("exit code %d, stderr:\n%s", code, stderr)
	}
	for _, want := range []string{
		"Running study with 50 bootstrap samples",
		"for the normal population",
		"for the right-skewed population",
		"for the left-skewed population",
		"Samples of size n = 5",
		"Samples of size n = 9",
		"SE Median, standard bootstrap:",
		"----- Run Complete -----",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("study report is missing %q", want)
		}
	}
}

func TestRunStudyExplainsMissingFiles(t *testing.T) {
	code, _, stderr := runProgram("-study", t.TempDir())
	if code != exitFailure || !strings.Contains(stderr, "Rscript R/generate-data.R") {
		t.Errorf("exit code %d, stderr should tell the user how to create the files:\n%s", code, stderr)
	}
}

func TestRound2PrintsLikeR(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{2, "2"},
		{0.5, "0.5"},
		{1.049, "1.05"},
		{0.6667, "0.67"},
	}
	for _, tt := range tests {
		if got := round2(tt.value); got != tt.want {
			t.Errorf("round2(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestBootstrapStudyGivesTheSameResultsForAnyNumberOfWorkers(t *testing.T) {
	path := writeFile(t, t.TempDir(), "normal.csv", smallStudyFile())
	samples, err := readStudySamples(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := bootstrapStudy(samples, 50, 1, 9999)
	if err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{2, 3, 16} {
		got, err := bootstrapStudy(samples, 50, workers, 9999)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%d workers gave different study results than 1 worker", workers)
		}
	}
}
