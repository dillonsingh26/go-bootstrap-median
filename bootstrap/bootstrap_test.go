package bootstrap

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"slices"
	"testing"
)

func TestRunRejectsUnusableInput(t *testing.T) {
	data := []float64{1, 2, 3}
	tests := []struct {
		name       string
		data       []float64
		resamples  int
		useNilRNG  bool
		statistics []Statistic
		wantErr    error
	}{
		{"empty data", nil, 100, false, []Statistic{Median}, ErrNoData},
		{"one resample", data, 1, false, []Statistic{Median}, ErrTooFewResamples},
		{"nil random number generator", data, 100, true, []Statistic{Median}, ErrNoRandomSource},
		{"no statistic", data, 100, false, nil, ErrNoStatistics},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rng := NewMersenneTwister(1)
			if tt.useNilRNG {
				rng = nil
			}
			_, err := Run(tt.data, tt.resamples, rng, tt.statistics...)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Run error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunParallelRejectsUnusableInput(t *testing.T) {
	data := []float64{1, 2, 3}
	tests := []struct {
		name      string
		data      []float64
		resamples int
		workers   int
		wantErr   error
	}{
		{"empty data", nil, 100, 4, ErrNoData},
		{"one resample", data, 1, 4, ErrTooFewResamples},
		{"no workers", data, 100, 0, ErrNoWorkers},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RunParallel(tt.data, tt.resamples, tt.workers, 1, Median)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("RunParallel error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Any number of workers must give identical results (run with -race to also check memory safety).
func TestRunParallelGivesTheSameResultsForAnyNumberOfWorkers(t *testing.T) {
	data := randomValues(501)
	for _, resamples := range []int{10, 1000} {
		want, err := RunParallel(data, resamples, 1, 9999, Mean, MedianInPlace)
		if err != nil {
			t.Fatal(err)
		}
		for _, workers := range []int{2, 7, 64, 100} {
			got, err := RunParallel(data, resamples, workers, 9999, Mean, MedianInPlace)
			if err != nil {
				t.Fatal(err)
			}
			for i := range want {
				if !slices.Equal(got[i].Replicates, want[i].Replicates) {
					t.Fatalf("B=%d: %d workers gave different replicates than 1 worker",
						resamples, workers)
				}
			}
		}
	}
}

func TestRunParallelStandardErrorOfMedianMatchesTheory(t *testing.T) {
	data := randomValues(10000)
	results, err := RunParallel(data, 1000, 4, 1, MedianInPlace)
	if err != nil {
		t.Fatal(err)
	}
	want := 10 * math.Sqrt(math.Pi/2) / math.Sqrt(float64(len(data)))
	got := results[0].StandardError
	t.Logf("parallel bootstrap SE of the median %.4f, theory %.4f", got, want)
	if !closeTo(got, want, 0.15) {
		t.Errorf("bootstrap SE of the median = %.4f, want within 15%% of %.4f", got, want)
	}
}

func TestRunResultShape(t *testing.T) {
	data := randomValues(50)
	results, err := Run(data, 200, NewMersenneTwister(1), Mean, Median)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want one per statistic (2)", len(results))
	}
	if results[0].Estimate != Mean(data) || results[1].Estimate != Median(data) {
		t.Errorf("estimates %v and %v should be the mean and median of the data",
			results[0].Estimate, results[1].Estimate)
	}
	for _, result := range results {
		if len(result.Replicates) != 200 {
			t.Errorf("got %d replicates, want 200", len(result.Replicates))
		}
	}
}

// Constant data never varies, so its standard error must be exactly 0.
func TestRunOnConstantDataHasZeroStandardError(t *testing.T) {
	results, err := Run([]float64{5, 5, 5, 5}, 100, NewMersenneTwister(1), Median)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Estimate != 5 || results[0].StandardError != 0 {
		t.Errorf("got estimate %v and standard error %v, want 5 and 0",
			results[0].Estimate, results[0].StandardError)
	}
}

func TestRunIsReproducibleWithTheSameSeed(t *testing.T) {
	data := randomValues(100)
	first, _ := Run(data, 100, NewMersenneTwister(42), Median)
	second, _ := Run(data, 100, NewMersenneTwister(42), Median)
	if !slices.Equal(first[0].Replicates, second[0].Replicates) {
		t.Error("two runs with the same seed gave different replicates")
	}
}

func TestRunNeverChangesTheData(t *testing.T) {
	data := randomValues(101)
	original := slices.Clone(data)
	if _, err := Run(data, 50, NewMersenneTwister(1), MedianInPlace); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(data, original) {
		t.Error("Run reordered the caller's data")
	}
}

func TestFillResampleUsesOnlyValuesFromTheData(t *testing.T) {
	data := []float64{1, 2, 3}
	sample := make([]float64, 300)
	fillResample(sample, slices.Repeat(data, 100), NewMersenneTwister(1))
	counts := map[float64]int{}
	for _, value := range sample {
		counts[value]++
	}
	if len(counts) != len(data) {
		t.Errorf("resample has values %v, want exactly the values %v", counts, data)
	}
}

// Theory: the bootstrap SE of the mean is about SD / sqrt(n).
func TestStandardErrorOfMeanMatchesTheory(t *testing.T) {
	data := randomValues(1000)
	results, err := Run(data, 2000, NewMersenneTwister(1), Mean)
	if err != nil {
		t.Fatal(err)
	}
	want := StandardDeviation(data) / math.Sqrt(float64(len(data)))
	got := results[0].StandardError
	t.Logf("bootstrap SE of the mean %.4f, theory %.4f", got, want)
	if !closeTo(got, want, 0.05) {
		t.Errorf("bootstrap SE of the mean = %.4f, want within 5%% of %.4f", got, want)
	}
}

// Theory for normal data: SE of the median is about 10 * sqrt(pi/2) / sqrt(n).
// Single bootstrap SEs of the median are noisy, so the tolerance is wider.
func TestStandardErrorOfMedianMatchesTheory(t *testing.T) {
	data := randomValues(10000)
	results, err := Run(data, 1000, NewMersenneTwister(1), Median)
	if err != nil {
		t.Fatal(err)
	}
	want := 10 * math.Sqrt(math.Pi/2) / math.Sqrt(float64(len(data)))
	got := results[0].StandardError
	t.Logf("bootstrap SE of the median %.4f, theory %.4f", got, want)
	if !closeTo(got, want, 0.15) {
		t.Errorf("bootstrap SE of the median = %.4f, want within 15%% of %.4f", got, want)
	}
}

func TestQuantileMatchesR(t *testing.T) {
	oneToTen := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	tests := []struct {
		p    float64
		want float64 // from R: quantile(1:10, p)
	}{
		{0, 1},
		{0.25, 3.25},
		{0.5, 5.5},
		{0.95, 9.55},
		{1, 10},
	}
	for _, tt := range tests {
		if got := quantile(oneToTen, tt.p); !closeTo(got, tt.want, 1e-12) {
			t.Errorf("quantile(1:10, %v) = %v, want %v", tt.p, got, tt.want)
		}
	}
}

func TestPercentileInterval(t *testing.T) {
	// Replicates 1 to 101: a 90% interval runs from 6 to 96 (tiny tolerance for rounding).
	result := Result{Replicates: make([]float64, 101)}
	for i := range result.Replicates {
		result.Replicates[i] = float64(101 - i)
	}
	low, high := result.PercentileInterval(0.90)
	if !closeTo(low, 6, 1e-12) || !closeTo(high, 96, 1e-12) {
		t.Errorf("90%% interval = [%v, %v], want [6, 96]", low, high)
	}
	if low, high := result.PercentileInterval(1.5); !math.IsNaN(low) || !math.IsNaN(high) {
		t.Errorf("interval at level 1.5 = [%v, %v], want NaN", low, high)
	}
}

func ExampleRun() {
	data := []float64{3, 5, 7, 9, 20}
	results, err := Run(data, 1000, NewMersenneTwister(9999), Median)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("median %v from %d bootstrap samples\n",
		results[0].Estimate, len(results[0].Replicates))
	// Output: median 7 from 1000 bootstrap samples
}

func BenchmarkRunMedian(b *testing.B) {
	const resamples = 100
	for _, n := range []int{100, 1000, 10000, 100000} {
		data := randomValues(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				Run(data, resamples, NewMersenneTwister(1), MedianInPlace)
			}
		})
	}
}

func BenchmarkRunParallelMedian(b *testing.B) {
	const resamples = 100
	for _, n := range []int{100, 1000, 10000, 100000} {
		data := randomValues(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				RunParallel(data, resamples, runtime.NumCPU(), 1, MedianInPlace)
			}
		})
	}
}
