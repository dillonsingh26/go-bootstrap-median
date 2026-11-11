//go:build unix

// Command compare times every R and Go version on the same data: wall time, CPU time, peak memory.
// Run from the project root with "make compare"; results go to results/.
package main

import (
	"cmp"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// benchmarkCase is one program run on one workload.
type benchmarkCase struct {
	workload string   // what is computed, such as "Monte Carlo study, B=100"
	program  string   // which implementation, such as "R original"
	command  []string // the command line that runs it
	slow     bool     // run only once, because it takes minutes
}

// measurement is the time and memory of one finished run.
type measurement struct {
	wallSeconds float64
	cpuSeconds  float64 // user plus system time, summed over all cores
	peakMB      float64 // maximum resident memory
}

const goProgram = "./bootmedian"

func main() {
	repeats := flag.Int("repeats", 3, "times to run each case (slow cases run once); the median is reported")
	skipSlow := flag.Bool("skip-slow", false, "skip the original R script at B=1000, which takes several minutes")
	tableOnly := flag.Bool("table-only", false, "rebuild results/comparison.md from results/comparison.csv without rerunning")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if *tableOnly {
		summary, repeats, err := readSummary(filepath.Join("results", "comparison.csv"))
		if err == nil {
			err = writeMarkdown(filepath.Join("results", "comparison.md"), summary, repeats)
		}
		if err != nil {
			logger.Error("rebuilding the table failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*repeats, *skipSlow, logger); err != nil {
		logger.Error("comparison failed", "error", err)
		os.Exit(1)
	}
}

func run(repeats int, skipSlow bool, logger *slog.Logger) error {
	if repeats < 1 {
		return fmt.Errorf("-repeats must be at least 1, got %d", repeats)
	}
	if _, err := os.Stat(goProgram); err != nil {
		return fmt.Errorf("%s not found; build it first with: go build -o bootmedian . (%w)", goProgram, err)
	}
	if err := os.MkdirAll(filepath.Join("results", "output"), 0o755); err != nil {
		return err
	}

	csvFile, err := os.Create(filepath.Join("results", "comparison.csv"))
	if err != nil {
		return err
	}
	defer csvFile.Close()
	runLog := csv.NewWriter(csvFile)
	runLog.Write([]string{"workload", "program", "run", "wall_seconds", "cpu_seconds", "peak_mb"})

	var summary []caseSummary
	for _, c := range benchmarkCases() {
		if c.slow && skipSlow {
			continue
		}
		runs := repeats
		if c.slow {
			runs = 1
		}
		var measurements []measurement
		for i := range runs {
			logger.Info("running", "workload", c.workload, "program", c.program, "run", i+1, "of", runs)
			m, err := measure(c.command, outputPath(c))
			if err != nil {
				return fmt.Errorf("%s, %s: %w", c.workload, c.program, err)
			}
			measurements = append(measurements, m)
			runLog.Write([]string{c.workload, c.program, strconv.Itoa(i + 1),
				formatFloat(m.wallSeconds), formatFloat(m.cpuSeconds), formatFloat(m.peakMB)})
		}
		summary = append(summary, summarize(c, measurements))
	}
	runLog.Flush()
	if err := runLog.Error(); err != nil {
		return err
	}
	return writeMarkdown(filepath.Join("results", "comparison.md"), summary, repeats)
}

// benchmarkCases lists every run; the first in each workload is the R reference.
func benchmarkCases() []benchmarkCase {
	var cases []benchmarkCase
	for _, resamples := range []string{"100", "1000"} {
		workload := "Monte Carlo study, B=" + resamples
		cases = append(cases,
			benchmarkCase{workload, "R original", []string{"Rscript", "R/run-bootstrap-median.R", resamples}, resamples == "1000"},
			benchmarkCase{workload, "R optimized", []string{"Rscript", "R/bootstrap-fast.R", resamples}, false},
			benchmarkCase{workload, "Go, 1 core", []string{goProgram, "-study", "data/study", "-B", resamples, "-workers", "1"}, false},
			benchmarkCase{workload, "Go, all cores", []string{goProgram, "-study", "data/study", "-B", resamples}, false},
		)
	}
	for _, n := range []string{"1000", "10000", "100000"} {
		workload := "one dataset, n=" + n + ", B=1000"
		path := "data/timing/normal-n" + n + ".csv"
		cases = append(cases,
			benchmarkCase{workload, "R boot package", []string{"Rscript", "R/bootstrap-boot.R", "1000", path}, false},
			benchmarkCase{workload, "Go, 1 core", []string{goProgram, "-file", path, "-B", "1000", "-workers", "1"}, false},
			benchmarkCase{workload, "Go, all cores", []string{goProgram, "-file", path, "-B", "1000"}, false},
		)
	}
	return cases
}

// measure runs a command, saves its output, and returns its time and memory.
func measure(command []string, outputPath string) (measurement, error) {
	output, err := os.Create(outputPath)
	if err != nil {
		return measurement{}, err
	}
	defer output.Close()

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = output
	cmd.Stderr = output
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return measurement{}, fmt.Errorf("%s failed (see %s): %w", strings.Join(command, " "), outputPath, err)
	}
	wall := time.Since(start)

	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return measurement{}, errors.New("this system does not report memory use")
	}
	cpu := cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
	return measurement{
		wallSeconds: wall.Seconds(),
		cpuSeconds:  cpu.Seconds(),
		peakMB:      peakBytes(usage) / (1 << 20),
	}, nil
}

// peakBytes converts peak memory to bytes (macOS reports bytes, Linux kilobytes).
func peakBytes(usage *syscall.Rusage) float64 {
	if runtime.GOOS == "darwin" {
		return float64(usage.Maxrss)
	}
	return float64(usage.Maxrss) * 1024
}

// outputPath names the file that keeps one run's printed output.
func outputPath(c benchmarkCase) string {
	name := strings.NewReplacer(" ", "-", ",", "", "=", "", ".", "").Replace(
		strings.ToLower(c.workload + "-" + c.program))
	return filepath.Join("results", "output", name+".txt")
}

// caseSummary is the median measurement for one case.
type caseSummary struct {
	benchmarkCase
	runs int
	measurement
}

// summarize takes the median of each measurement, so one slow run can't skew it.
func summarize(c benchmarkCase, measurements []measurement) caseSummary {
	median := func(value func(measurement) float64) float64 {
		values := make([]float64, len(measurements))
		for i, m := range measurements {
			values[i] = value(m)
		}
		slices.Sort(values)
		middle := len(values) / 2
		if len(values)%2 == 1 {
			return values[middle]
		}
		return (values[middle-1] + values[middle]) / 2
	}
	return caseSummary{
		benchmarkCase: c,
		runs:          len(measurements),
		measurement: measurement{
			wallSeconds: median(func(m measurement) float64 { return m.wallSeconds }),
			cpuSeconds:  median(func(m measurement) float64 { return m.cpuSeconds }),
			peakMB:      median(func(m measurement) float64 { return m.peakMB }),
		},
	}
}

// writeMarkdown writes the summary table: least CPU time first within each workload, best row in bold.
func writeMarkdown(path string, summary []caseSummary, repeats int) error {
	var text strings.Builder
	fmt.Fprintf(&text, "# R vs. Go comparison\n\n")
	fmt.Fprintf(&text, "Machine: %s, %s/%s, %d cores. %s; %s.\n\n",
		commandOutput("sysctl", "-n", "machdep.cpu.brand_string"), runtime.GOOS, runtime.GOARCH,
		runtime.NumCPU(), runtime.Version(), commandOutput("Rscript", "-e", "cat(R.version.string)"))
	fmt.Fprintf(&text, "Medians of %d runs (slow cases run once). Times include starting the program and reading the data. ", repeats)
	fmt.Fprintf(&text, "CPU time is summed over all cores. Within each workload, rows run from least to most CPU time; the best is in bold. ")
	fmt.Fprintf(&text, "The last three columns compare each row with the workload's R reference (R original or R boot package).\n\n")
	fmt.Fprintf(&text, "| Workload | Program | Runs | Wall time (s) | CPU time (s) | Peak memory (MB) | Faster than R | Less CPU than R | Less memory than R |\n")
	fmt.Fprintf(&text, "|---|---|---:|---:|---:|---:|---:|---:|---:|\n")

	// The R reference is the first case listed for each workload.
	references := map[string]caseSummary{}
	for _, s := range summary {
		if _, found := references[s.workload]; !found {
			references[s.workload] = s
		}
	}
	sorted := sortByCPUTime(summary)
	for i, s := range sorted {
		reference := references[s.workload]
		program := s.program
		if i == 0 || s.workload != sorted[i-1].workload {
			program = "**" + program + "**"
		}
		fmt.Fprintf(&text, "| %s | %s | %d | %.3f | %.3f | %.0f | %.1f× | %.1f× | %.1f× |\n",
			s.workload, program, s.runs, s.wallSeconds, s.cpuSeconds, s.peakMB,
			reference.wallSeconds/s.wallSeconds, reference.cpuSeconds/s.cpuSeconds,
			reference.peakMB/s.peakMB)
	}
	return os.WriteFile(path, []byte(text.String()), 0o644)
}

// sortByCPUTime keeps workloads in their original order and sorts each one by CPU time, least first.
func sortByCPUTime(summary []caseSummary) []caseSummary {
	workloadOrder := map[string]int{}
	for i, s := range summary {
		if _, found := workloadOrder[s.workload]; !found {
			workloadOrder[s.workload] = i
		}
	}
	sorted := slices.Clone(summary)
	slices.SortStableFunc(sorted, func(a, b caseSummary) int {
		if order := cmp.Compare(workloadOrder[a.workload], workloadOrder[b.workload]); order != 0 {
			return order
		}
		return cmp.Compare(a.cpuSeconds, b.cpuSeconds)
	})
	return sorted
}

// readSummary rebuilds the summary from a saved comparison.csv, so the table can be redone without rerunning.
func readSummary(path string) ([]caseSummary, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("%w (run the comparison first)", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, 0, err
	}

	type caseName struct{ workload, program string }
	var cases []caseName
	runs := map[caseName][]measurement{}
	for _, record := range records[1:] {
		c := caseName{workload: record[0], program: record[1]}
		values := make([]float64, 3)
		for i, text := range record[3:6] {
			if values[i], err = strconv.ParseFloat(text, 64); err != nil {
				return nil, 0, fmt.Errorf("%s: %w", path, err)
			}
		}
		if _, seen := runs[c]; !seen {
			cases = append(cases, c)
		}
		runs[c] = append(runs[c], measurement{wallSeconds: values[0], cpuSeconds: values[1], peakMB: values[2]})
	}

	var summary []caseSummary
	repeats := 1
	for _, c := range cases {
		summary = append(summary, summarize(benchmarkCase{workload: c.workload, program: c.program}, runs[c]))
		repeats = max(repeats, len(runs[c]))
	}
	return summary, repeats, nil
}

// commandOutput returns what a command prints, or "unknown" if it fails.
func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}
