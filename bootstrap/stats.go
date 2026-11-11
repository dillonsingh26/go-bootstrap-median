package bootstrap

import (
	"math"
	"slices"
)

// Mean returns the average (NaN if empty).
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// StandardDeviation returns the sample SD, dividing by n-1 like R's sd().
func StandardDeviation(values []float64) float64 {
	if len(values) < 2 {
		return math.NaN()
	}
	mean := Mean(values)
	sumOfSquares := 0.0
	for _, value := range values {
		deviation := value - mean
		sumOfSquares += deviation * deviation
	}
	return math.Sqrt(sumOfSquares / float64(len(values)-1))
}

// Median returns the middle value, like R's median(), without changing values.
func Median(values []float64) float64 {
	return MedianInPlace(slices.Clone(values))
}

// MedianInPlace is a faster Median: no copy, no full sort, but it reorders values.
func MedianInPlace(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	middle := len(values) / 2
	selectKth(values, middle)
	if len(values)%2 == 1 {
		return values[middle]
	}
	// Even count: average the middle value with the largest value before it.
	return (slices.Max(values[:middle]) + values[middle]) / 2
}

// selectKth moves the k-th smallest value to position k (quickselect: Hoare 1961, Wirth 1976).
func selectKth(values []float64, k int) {
	left, right := 0, len(values)-1
	for left < right {
		pivot := values[k]
		i, j := left, right
		for i <= j {
			for values[i] < pivot {
				i++
			}
			for pivot < values[j] {
				j--
			}
			if i <= j {
				values[i], values[j] = values[j], values[i]
				i++
				j--
			}
		}
		// Left part <= pivot <= right part; keep only the part holding k.
		if j < k {
			left = i
		}
		if k < i {
			right = j
		}
	}
}
