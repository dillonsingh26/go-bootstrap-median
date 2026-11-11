// Package bootstrap estimates standard errors by resampling the data with replacement.
package bootstrap

import (
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"sync"

	"github.com/seehuhn/mt19937"
)

// Statistic turns a sample into one number, such as its median.
// It may reorder the sample but not change it, and must be safe on several cores at once.
type Statistic func(sample []float64) float64

// Result is the bootstrap estimate for one statistic.
type Result struct {
	Estimate      float64   // the statistic computed on the original data
	StandardError float64   // standard deviation of the replicates
	Replicates    []float64 // the statistic computed on each bootstrap sample
}

// Errors for unusable input.
var (
	ErrNoData          = errors.New("bootstrap: data is empty")
	ErrTooFewResamples = errors.New("bootstrap: need at least 2 resamples")
	ErrNoRandomSource  = errors.New("bootstrap: random number generator is nil")
	ErrNoStatistics    = errors.New("bootstrap: no statistic given")
	ErrNoWorkers       = errors.New("bootstrap: need at least 1 worker")
)

// chunkCount is how many blocks RunParallel splits the work into, each with its own seed.
// That keeps results the same for any number of cores.
const chunkCount = 64

// NewMersenneTwister returns a seeded Mersenne Twister random number generator (the algorithm family R uses).
func NewMersenneTwister(seed int64) *rand.Rand {
	source := mt19937.New()
	source.Seed(seed)
	return rand.New(source)
}

// Run computes each statistic on B bootstrap samples, using one core.
// It returns one Result per statistic and never changes data.
func Run(data []float64, resamples int, rng *rand.Rand, statistics ...Statistic) ([]Result, error) {
	if err := checkInput(data, resamples, statistics); err != nil {
		return nil, err
	}
	if rng == nil {
		return nil, ErrNoRandomSource
	}
	run := newReplicator(data, resamples, statistics)
	run.fill(0, resamples, make([]float64, len(data)), rng)
	return run.finish(), nil
}

// RunParallel is Run spread across workers cores; the same seed gives the same results for any number of workers.
func RunParallel(data []float64, resamples, workers int, seed int64, statistics ...Statistic) ([]Result, error) {
	if err := checkInput(data, resamples, statistics); err != nil {
		return nil, err
	}
	if workers < 1 {
		return nil, ErrNoWorkers
	}
	run := newReplicator(data, resamples, statistics)

	type chunk struct {
		first, last int   // replicate positions first to last-1
		seed        int64 // seed for this chunk's random number generator
	}
	chunks := make(chan chunk)
	var wg sync.WaitGroup
	for range min(workers, chunkCount) {
		// Each worker reuses one buffer and writes only to its own result slots.
		wg.Go(func() {
			sample := make([]float64, len(data))
			for c := range chunks {
				run.fill(c.first, c.last, sample, NewMersenneTwister(c.seed))
			}
		})
	}

	seeds := NewMersenneTwister(seed)
	for i := range chunkCount {
		chunks <- chunk{
			first: i * resamples / chunkCount,
			last:  (i + 1) * resamples / chunkCount,
			seed:  seeds.Int64(),
		}
	}
	close(chunks)
	wg.Wait()
	return run.finish(), nil
}

// checkInput returns the first problem with the input, if any.
func checkInput(data []float64, resamples int, statistics []Statistic) error {
	switch {
	case len(data) == 0:
		return ErrNoData
	case resamples < 2:
		return ErrTooFewResamples
	case len(statistics) == 0:
		return ErrNoStatistics
	}
	return nil
}

// replicator holds the data and results for one bootstrap run.
type replicator struct {
	data       []float64
	statistics []Statistic
	results    []Result
}

// newReplicator computes each estimate on a copy of the data and makes room for B results.
func newReplicator(data []float64, resamples int, statistics []Statistic) *replicator {
	copied := slices.Clone(data)
	results := make([]Result, len(statistics))
	for i, statistic := range statistics {
		results[i] = Result{
			Estimate:   statistic(copied),
			Replicates: make([]float64, resamples),
		}
	}
	return &replicator{data: data, statistics: statistics, results: results}
}

// fill computes replicates first to last-1, reusing sample for each bootstrap sample.
func (r *replicator) fill(first, last int, sample []float64, rng *rand.Rand) {
	for b := first; b < last; b++ {
		fillResample(sample, r.data, rng)
		for i, statistic := range r.statistics {
			r.results[i].Replicates[b] = statistic(sample)
		}
	}
}

// finish sets each standard error (the SD of the replicates) and returns the results.
func (r *replicator) finish() []Result {
	for i := range r.results {
		r.results[i].StandardError = StandardDeviation(r.results[i].Replicates)
	}
	return r.results
}

// PercentileInterval returns the range holding the middle level (e.g. 0.95) of the replicates.
// NaN if level is not between 0 and 1.
func (r Result) PercentileInterval(level float64) (low, high float64) {
	if level <= 0 || level >= 1 || len(r.Replicates) == 0 {
		return math.NaN(), math.NaN()
	}
	tail := (1 - level) / 2
	sorted := slices.Sorted(slices.Values(r.Replicates))
	return quantile(sorted, tail), quantile(sorted, 1-tail)
}

// quantile returns the p-th quantile of sorted values, like R's quantile().
func quantile(sorted []float64, p float64) float64 {
	position := p * float64(len(sorted)-1)
	lower := int(position)
	if lower >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	fraction := position - float64(lower)
	return sorted[lower] + fraction*(sorted[lower+1]-sorted[lower])
}

// fillResample fills sample with values picked at random from data, with replacement.
func fillResample(sample, data []float64, rng *rand.Rand) {
	for i := range sample {
		sample[i] = data[rng.IntN(len(data))]
	}
}
