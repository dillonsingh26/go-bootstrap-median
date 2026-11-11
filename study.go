package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/dillonsingh26/go-bootstrap-median/bootstrap"
)

// Mean and SD of every population in R/generate-data.R.
const (
	populationMean = 100
	populationSD   = 10
)

// studyShapes lists the study files (<shape>.csv) in the order R reports them.
var studyShapes = []string{"normal", "right-skewed", "left-skewed"}

// studySample is one sample of size n from a study file.
type studySample struct {
	n      int
	values []float64
}

// sampleSummary holds one sample's bootstrap results, matching R/bootstrap-fast.R.
type sampleSummary struct {
	n              int
	sampleMean     float64
	bootMean       float64 // average of the B bootstrap means
	bootMedian     float64 // average of the B bootstrap medians
	bootSEOfMean   float64 // SD of the B bootstrap means
	bootSEOfMedian float64 // SD of the B bootstrap medians
}

// runStudy reruns the R study for every shape and prints the same report.
func runStudy(cfg config, stdout io.Writer, logger *slog.Logger) error {
	fmt.Fprintf(stdout, "\nRunning study with %d bootstrap samples\n", cfg.resamples)
	fmt.Fprintf(stdout, "\nStudy conditions:\n  Population mean: %d SD: %d \n",
		populationMean, populationSD)

	start := time.Now()
	for _, shape := range studyShapes {
		path := filepath.Join(cfg.studyDir, shape+".csv")
		samples, err := readStudySamples(path)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w\nCreate the study files first with: Rscript R/generate-data.R", err)
		}
		if err != nil {
			return err
		}
		logger.Info("read study file", "path", path, "samples", len(samples))

		shapeStart := time.Now()
		summaries, err := bootstrapStudy(samples, cfg.resamples, cfg.workers, cfg.seed)
		if err != nil {
			return err
		}
		logger.Info("study shape finished", "shape", shape, "duration", time.Since(shapeStart))
		reportStudy(stdout, shape, cfg.resamples, summaries)
	}

	fmt.Fprintf(stdout, "\nElapsed time for all three shapes: %.2f seconds\n",
		time.Since(start).Seconds())
	fmt.Fprintf(stdout, "\n----- Run Complete -----\n")
	return nil
}

// readStudySamples reads a study file and groups its rows into samples.
func readStudySamples(path string) ([]studySample, error) {
	columns, err := readColumns(path, "iteration", "n", "value")
	if err != nil {
		return nil, err
	}
	iterations, sizes, values := columns[0], columns[1], columns[2]

	var samples []studySample
	start := 0
	for end := 1; end <= len(values); end++ {
		sameSample := end < len(values) &&
			iterations[end] == iterations[start] && sizes[end] == sizes[start]
		if sameSample {
			continue
		}
		n := int(sizes[start])
		if end-start != n {
			return nil, fmt.Errorf("%s: the sample for iteration %v has %d values, but its n column says %d",
				path, iterations[start], end-start, n)
		}
		samples = append(samples, studySample{n: n, values: values[start:end]})
		start = end
	}
	return samples, nil
}

// bootstrapStudy bootstraps every sample, spreading the samples across workers.
// Each sample has its own seed, so results don't depend on the number of workers.
func bootstrapStudy(samples []studySample, resamples, workers int, seed int64) ([]sampleSummary, error) {
	seeds := bootstrap.NewMersenneTwister(seed)
	sampleSeeds := make([]int64, len(samples))
	for i := range sampleSeeds {
		sampleSeeds[i] = seeds.Int64()
	}

	summaries := make([]sampleSummary, len(samples))
	errs := make([]error, len(samples))
	positions := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range positions {
				summaries[i], errs[i] = summarizeSample(samples[i], resamples, sampleSeeds[i])
			}
		})
	}
	for i := range samples {
		positions <- i
	}
	close(positions)
	wg.Wait()
	return summaries, errors.Join(errs...)
}

// summarizeSample bootstraps the mean and median of one study sample.
func summarizeSample(sample studySample, resamples int, seed int64) (sampleSummary, error) {
	rng := bootstrap.NewMersenneTwister(seed)
	results, err := bootstrap.Run(sample.values, resamples, rng, bootstrap.Mean, bootstrap.MedianInPlace)
	if err != nil {
		return sampleSummary{}, err
	}
	means, medians := results[0], results[1]
	return sampleSummary{
		n:              sample.n,
		sampleMean:     means.Estimate,
		bootMean:       bootstrap.Mean(means.Replicates),
		bootMedian:     bootstrap.Mean(medians.Replicates),
		bootSEOfMean:   means.StandardError,
		bootSEOfMedian: medians.StandardError,
	}, nil
}

// reportStudy prints the standard errors in the same format as R/bootstrap-fast.R.
func reportStudy(w io.Writer, shape string, resamples int, summaries []sampleSummary) {
	fmt.Fprintf(w, "\nEstimated standard errors for the %s population using %d bootstrap samples\n",
		shape, resamples)
	for _, n := range sampleSizes(summaries) {
		var sampleMeans, bootMeans, bootMedians, seOfMeans, seOfMedians []float64
		for _, summary := range summaries {
			if summary.n != n {
				continue
			}
			sampleMeans = append(sampleMeans, summary.sampleMean)
			bootMeans = append(bootMeans, summary.bootMean)
			bootMedians = append(bootMedians, summary.bootMedian)
			seOfMeans = append(seOfMeans, summary.bootSEOfMean)
			seOfMedians = append(seOfMedians, summary.bootSEOfMedian)
		}
		fmt.Fprintf(w, "\nSamples of size n = %d\n", n)
		fmt.Fprintf(w, "  SE Mean from Central Limit Theorem: %s\n", round2(populationSD/math.Sqrt(float64(n))))
		fmt.Fprintf(w, "  SE Mean from Samples: %s\n", round2(bootstrap.StandardDeviation(sampleMeans)))
		fmt.Fprintf(w, "  SE Mean from Bootstrap Samples: %s\n", round2(bootstrap.StandardDeviation(bootMeans)))
		fmt.Fprintf(w, "  SE Median from Bootstrap Samples: %s\n", round2(bootstrap.StandardDeviation(bootMedians)))
		fmt.Fprintf(w, "  SE Mean, standard bootstrap: %s\n", round2(bootstrap.Mean(seOfMeans)))
		fmt.Fprintf(w, "  SE Median, standard bootstrap: %s\n", round2(bootstrap.Mean(seOfMedians)))
	}
}

// sampleSizes lists the distinct sample sizes in the order they first appear.
func sampleSizes(summaries []sampleSummary) []int {
	var sizes []int
	for _, summary := range summaries {
		if !slices.Contains(sizes, summary.n) {
			sizes = append(sizes, summary.n)
		}
	}
	return sizes
}

// round2 prints x to 2 decimals the way R's cat(round(x, 2)) does, e.g. 2, 0.5, 1.05.
func round2(x float64) string {
	return strconv.FormatFloat(math.Round(x*100)/100, 'f', -1, 64)
}
