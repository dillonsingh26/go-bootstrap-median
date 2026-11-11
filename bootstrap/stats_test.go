package bootstrap

import (
	"fmt"
	"math"
	"slices"
	"testing"
)

// randomValues returns n normal values (mean 100, SD 10), the same every run.
func randomValues(n int) []float64 {
	rng := NewMersenneTwister(9999)
	values := make([]float64, n)
	for i := range values {
		values[i] = 100 + 10*rng.NormFloat64()
	}
	return values
}

// closeTo reports whether got is within a relative tolerance of want.
func closeTo(got, want, tolerance float64) bool {
	return math.Abs(got-want) <= tolerance*math.Abs(want)
}

func TestMean(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"single value", []float64{7}, 7},
		{"whole-number mean", []float64{1, 2, 3}, 2},
		{"fractional mean", []float64{1, 2, 3, 4}, 2.5},
		{"negative values", []float64{-4, -2}, -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Mean(tt.values); got != tt.want {
				t.Errorf("Mean(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestStandardDeviation(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		// Values from R: sd(c(2, 4, 4, 4, 5, 5, 7, 9)) and sd(c(1, 2, 3, 4))
		{"textbook example", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.138089935299395},
		{"consecutive integers", []float64{1, 2, 3, 4}, 1.2909944487358056},
		{"constant values", []float64{5, 5, 5}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StandardDeviation(tt.values); !closeTo(got, tt.want, 1e-12) {
				t.Errorf("StandardDeviation(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestStatisticsOfTooFewValuesAreNaN(t *testing.T) {
	if got := Mean(nil); !math.IsNaN(got) {
		t.Errorf("Mean(nil) = %v, want NaN", got)
	}
	if got := Median(nil); !math.IsNaN(got) {
		t.Errorf("Median(nil) = %v, want NaN", got)
	}
	if got := StandardDeviation([]float64{3}); !math.IsNaN(got) {
		t.Errorf("StandardDeviation of one value = %v, want NaN", got)
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"single value", []float64{7}, 7},
		{"odd count, sorted", []float64{1, 2, 3}, 2},
		{"odd count, unsorted", []float64{9, 1, 5, 3, 7}, 5},
		{"even count averages the middle two", []float64{4, 1, 3, 2}, 2.5},
		{"repeated values", []float64{5, 5, 5, 1, 9}, 5},
		{"negative values", []float64{-3, -1, -2}, -2},
		{"long right tail", []float64{3, 5, 7, 9, 20}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Median(tt.values); got != tt.want {
				t.Errorf("Median(%v) = %v, want %v", tt.values, got, tt.want)
			}
			if got := MedianInPlace(slices.Clone(tt.values)); got != tt.want {
				t.Errorf("MedianInPlace(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestMedianDoesNotChangeInput(t *testing.T) {
	values := []float64{3, 1, 2}
	original := slices.Clone(values)
	Median(values)
	if !slices.Equal(values, original) {
		t.Errorf("Median changed its input from %v to %v", original, values)
	}
}

// Half the values must be below the median and half above, for every n from 1 to 200.
func TestMedianSplitsRandomDataInHalf(t *testing.T) {
	for n := 1; n <= 200; n++ {
		values := randomValues(n)
		median := Median(values)
		below, above := 0, 0
		for _, value := range values {
			if value < median {
				below++
			} else if value > median {
				above++
			}
		}
		if below != above || below != n/2 {
			t.Fatalf("n=%d: %d values below the median and %d above, want %d each",
				n, below, above, n/2)
		}
	}
}

// sortedMedian is the simple sort-then-middle median, used to check the fast one.
func sortedMedian(values []float64) float64 {
	sorted := slices.Sorted(slices.Values(values))
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// The fast median must match the sorting median on random, tied, sorted, reversed, and constant data.
func TestMedianInPlaceMatchesSortedMedian(t *testing.T) {
	rng := NewMersenneTwister(7)
	for n := 1; n <= 300; n++ {
		random := randomValues(n)
		withTies := make([]float64, n)
		fillResample(withTies, random, rng)
		ascending := slices.Sorted(slices.Values(random))
		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		allEqual := slices.Repeat([]float64{4}, n)

		for _, values := range [][]float64{random, withTies, ascending, descending, allEqual} {
			want := sortedMedian(values)
			if got := MedianInPlace(slices.Clone(values)); got != want {
				t.Fatalf("n=%d: MedianInPlace = %v, sorting gives %v", n, got, want)
			}
		}
	}
}

// selectKth must put the k-th smallest value at k, smaller values before it, larger after.
func TestSelectKthPartitionsAroundK(t *testing.T) {
	data := randomValues(64)
	rng := NewMersenneTwister(3)
	fillResample(data, slices.Clone(data), rng) // add repeated values
	sorted := slices.Sorted(slices.Values(data))
	for k := range data {
		values := slices.Clone(data)
		selectKth(values, k)
		if values[k] != sorted[k] {
			t.Fatalf("k=%d: values[k] = %v, want %v", k, values[k], sorted[k])
		}
		if slices.Max(values[:k+1]) > values[k] || slices.Min(values[k:]) < values[k] {
			t.Fatalf("k=%d: values are not partitioned around position k", k)
		}
	}
}

func ExampleMedian() {
	fmt.Println(Median([]float64{3, 5, 7, 9, 20}))
	fmt.Println(Median([]float64{4, 1, 3, 2}))
	// Output:
	// 7
	// 2.5
}

func BenchmarkMedian(b *testing.B) {
	for _, n := range []int{1000, 10000, 100000} {
		values := randomValues(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				Median(values)
			}
		})
	}
}
