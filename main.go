// Command bootmedian bootstraps the standard error of the median for one CSV file or the R study.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/dillonsingh26/go-bootstrap-median/bootstrap"
)

// Exit codes: 0 success, 1 analysis failed, 2 bad options.
const (
	exitOK         = 0
	exitFailure    = 1
	exitUsageError = 2
)

const usageIntro = `bootmedian estimates the standard error of the median by bootstrap resampling.

Usage:
  bootmedian -file data/timing/normal-n10000.csv [options]   analyze one dataset
  bootmedian -study data/study [options]                     rerun the R Monte Carlo study

Options:
`

// errNoOptions means no options were given, so only the help was printed.
var errNoOptions = errors.New("no options given")

// config holds the checked command-line settings.
type config struct {
	filePath   string
	studyDir   string
	resamples  int
	seed       int64
	workers    int
	logPath    string
	cpuProfile string
	memProfile string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the whole program; taking args and outputs as parameters lets tests call it.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseFlags(args, stderr)
	switch {
	case errors.Is(err, flag.ErrHelp):
		return exitOK
	case errors.Is(err, errNoOptions):
		return exitUsageError
	case err != nil:
		fmt.Fprintf(stderr, "error: %v\nRun bootmedian -h to see the options.\n", err)
		return exitUsageError
	}

	logger, closeLog, err := newLogger(cfg.logPath, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitFailure
	}
	defer closeLog()

	if err := analyze(cfg, stdout, logger); err != nil {
		logger.Error("run failed", "error", err)
		fmt.Fprintln(stderr, "error:", err)
		return exitFailure
	}
	return exitOK
}

// parseFlags reads and checks the command-line options.
func parseFlags(args []string, stderr io.Writer) (config, error) {
	flags := flag.NewFlagSet("bootmedian", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	printUsage := func() {
		flags.SetOutput(stderr)
		fmt.Fprint(stderr, usageIntro)
		flags.PrintDefaults()
	}

	var cfg config
	flags.StringVar(&cfg.filePath, "file", "", "CSV `file` with a column named \"value\" to bootstrap")
	flags.StringVar(&cfg.studyDir, "study", "", "`folder` holding normal.csv, right-skewed.csv, and left-skewed.csv\nfrom R/generate-data.R")
	flags.IntVar(&cfg.resamples, "B", 1000, "number of bootstrap samples, at least 2")
	flags.Int64Var(&cfg.seed, "seed", 9999, "seed for the Mersenne Twister random number generator")
	flags.IntVar(&cfg.workers, "workers", runtime.NumCPU(), "number of CPU cores to use; results are the same for any number")
	flags.StringVar(&cfg.logPath, "log", "", "write progress logs to this `file` instead of standard error")
	flags.StringVar(&cfg.cpuProfile, "cpuprofile", "", "write a CPU profile to this `file`")
	flags.StringVar(&cfg.memProfile, "memprofile", "", "write a memory allocation profile to this `file`")

	if len(args) == 0 {
		printUsage()
		return config{}, errNoOptions
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage()
		}
		return config{}, err
	}

	switch {
	case flags.NArg() > 0:
		return config{}, fmt.Errorf("unexpected argument %q (options start with -)", flags.Arg(0))
	case cfg.filePath == "" && cfg.studyDir == "":
		return config{}, errors.New("give either -file or -study")
	case cfg.filePath != "" && cfg.studyDir != "":
		return config{}, errors.New("give -file or -study, not both")
	case cfg.resamples < 2:
		return config{}, fmt.Errorf("-B must be at least 2, got %d", cfg.resamples)
	case cfg.workers < 1:
		return config{}, fmt.Errorf("-workers must be at least 1, got %d", cfg.workers)
	}
	return cfg, nil
}

// newLogger logs to a file, or to stderr if no file is given.
func newLogger(path string, stderr io.Writer) (*slog.Logger, func(), error) {
	if path == "" {
		return slog.New(slog.NewTextHandler(stderr, nil)), func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("creating log file: %w", err)
	}
	return slog.New(slog.NewTextHandler(file, nil)), func() { file.Close() }, nil
}

// analyze runs the chosen mode, with profiling when it was requested.
func analyze(cfg config, stdout io.Writer, logger *slog.Logger) error {
	logger.Info("starting", "file", cfg.filePath, "study", cfg.studyDir,
		"resamples", cfg.resamples, "seed", cfg.seed, "workers", cfg.workers, "cpus", runtime.NumCPU())

	stopCPUProfile, err := startCPUProfile(cfg.cpuProfile)
	if err != nil {
		return err
	}
	defer stopCPUProfile()

	if cfg.studyDir != "" {
		err = runStudy(cfg, stdout, logger)
	} else {
		err = runFile(cfg, stdout, logger)
	}
	if err != nil {
		return err
	}

	logMemoryUse(logger)
	return writeMemProfile(cfg.memProfile)
}

// runFile bootstraps the median of one dataset and prints the results.
func runFile(cfg config, stdout io.Writer, logger *slog.Logger) error {
	readStart := time.Now()
	values, err := readValues(cfg.filePath)
	if err != nil {
		return err
	}
	logger.Info("read data", "path", cfg.filePath, "values", len(values),
		"duration", time.Since(readStart))

	bootstrapStart := time.Now()
	results, err := bootstrap.RunParallel(values, cfg.resamples, cfg.workers, cfg.seed, bootstrap.MedianInPlace)
	if err != nil {
		return err
	}
	elapsed := time.Since(bootstrapStart)
	logger.Info("bootstrap finished", "duration", elapsed)

	median := results[0]
	low, high := median.PercentileInterval(0.95)
	fmt.Fprintf(stdout, "Bootstrap estimate of the median for %s\n", cfg.filePath)
	fmt.Fprintf(stdout, "  values:              %d\n", len(values))
	fmt.Fprintf(stdout, "  bootstrap samples:   %d\n", cfg.resamples)
	fmt.Fprintf(stdout, "  median:              %.3f\n", median.Estimate)
	fmt.Fprintf(stdout, "  SE of the median:    %.4f\n", median.StandardError)
	fmt.Fprintf(stdout, "  95%% percentile CI:   [%.3f, %.3f]\n", low, high)
	fmt.Fprintf(stdout, "  bootstrap time:      %.3f seconds\n", elapsed.Seconds())
	return nil
}

// startCPUProfile starts CPU profiling if a file is given; call stop when done.
func startCPUProfile(path string) (stop func(), err error) {
	if path == "" {
		return func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating CPU profile: %w", err)
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("starting CPU profile: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		file.Close()
	}, nil
}

// writeMemProfile saves all memory allocations so far, if a file is given.
func writeMemProfile(path string) error {
	if path == "" {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating memory profile: %w", err)
	}
	defer file.Close()
	return pprof.Lookup("allocs").WriteTo(file, 0)
}

// logMemoryUse logs how much memory the program has used.
func logMemoryUse(logger *slog.Logger) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	const megabyte = 1 << 20
	logger.Info("memory use",
		"total_allocated_mb", stats.TotalAlloc/megabyte,
		"heap_reserved_mb", stats.HeapSys/megabyte,
		"from_os_mb", stats.Sys/megabyte,
		"garbage_collections", stats.NumGC)
}
