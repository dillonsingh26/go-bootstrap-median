# go-bootstrap-median

The same statistical method, a bootstrap estimate of the standard error of the median, built in R and in Go and compared on the same data for speed, memory, and cloud cost.

## The short version

**The question:** a research consultancy does its statistics in R and its cloud bills are growing. Would switching to Go save money, and how hard is the switch?

**The method tested:** bootstrapping, one of the key statistical ideas listed by Gelman and Vehtari (2021). It's used here to estimate how uncertain a median is, for normal, right-skewed, and left-skewed data.

**The answer:**

- **Same results.** Go and R give the same answers, and both match statistical theory.
- **Much faster.** For the same job, Go used far less CPU time than R:

  | Job | R | Go (1 core) | Go is… |
  |---|---|---|---|
  | Monte Carlo study, B = 1,000 (vs. Course Files R script) | 170 s | 2.6 s | **65× faster** |
  | Monte Carlo study, B = 1,000 (vs. optimized R) | 43 s | 2.6 s | **16× faster** |
  | One dataset, n = 100,000 (vs. R `boot` package) | 5.4 s | 1.0 s | **5× faster** |

- **Much less memory.** R's `boot` package needed about 1 GB for the biggest job. Go needed 11 MB.
- **Cheaper.** On AWS EC2, that cuts the computing cost of this method by **80% to 95%**.
- **The catch.** A straight copy of the R code into Go was *not* faster on big datasets. The speed came from profiling the code and making three specific fixes.
- **Recommendation:** move big, repeated jobs to Go. Keep exploratory and one-off work in R. [Details below.](#recommendation-to-the-consultancy)

## Contents

- [What's in this repository](#whats-in-this-repository)
- [Quick start](#quick-start)
- [Using bootmedian](#using-bootmedian)
- [Building the executable](#building-the-executable)
- [Testing, benchmarking, profiling, and logging](#testing-benchmarking-profiling-and-logging)
- [What bootstrapping is](#what-bootstrapping-is)
- [Finding R and Go packages](#finding-r-and-go-packages)
- [How the Go version was built](#how-the-go-version-was-built)
- [Results](#results)
- [What I learned from testing, benchmarking, profiling, and logging](#what-i-learned-from-testing-benchmarking-profiling-and-logging)
- [Recommendation to the consultancy](#recommendation-to-the-consultancy)
- [GenAI Tools](#genai-tools)
- [References](#references)

## What's in this repository

| Path | What it is |
|---|---|
| `R/generate-data.R` | Makes all the datasets in `data/`, so R and Go use the exact same numbers |
| `R/run-bootstrap-median.R` | The bootstrap script from the Course Files, changed as little as possible (each change marked `CHANGED:`). This is the slow R starting point |
| `R/bootstrap-fast.R` | The same script, rewritten to run fast. Same results, so R gets a fair comparison |
| `R/bootstrap-boot.R` | The same estimate using R's `boot` package from CRAN |
| `bootstrap/` | The Go bootstrap package. Reusable in other projects |
| `main.go`, `data.go`, `study.go` | The `bootmedian` program: options and logging, reading CSV files, and the Monte Carlo study |
| `*_test.go` | 37 tests and examples |
| `cmd/compare/` | A Go tool that times every R and Go version and records CPU time and memory |
| `data/study/` | Monte Carlo study data: 100 samples each of size 25, 100, 225, and 400, for each of the three shapes |
| `data/timing/` | One large sample (n = 1,000, 10,000, and 100,000) for each shape |
| `results/` | Timing results from `cmd/compare`, plus each program's printed output |
| `docs/benchmarks/` | Go benchmark results before and after each speed fix |
| `docs/profiles/` | Profiling summaries showing where the time went |
| `docs/logs/` | Example program logs |
| `dist/` | Ready-to-run programs for Mac, Windows, and Linux |
| `Makefile` | Shortcuts for every command below |

## Quick start

You need **Go 1.25 or newer** ([go.dev/dl](https://go.dev/dl/)). R is only needed to run the R scripts; the data are already included.

```bash
git clone https://github.com/dillonsingh26/go-bootstrap-median.git
cd go-bootstrap-median
make            # check the code, run all tests, and build ./bootmedian
./bootmedian -file data/timing/normal-n10000.csv -B 1000
./bootmedian -study data/study -B 100
```

No `make` (for example on Windows)? Use `go test ./...` to test and `go build -o bootmedian.exe .` to build.

To run the R scripts, from the project folder:

```bash
Rscript R/generate-data.R            # rebuild the data (same files every time)
Rscript R/run-bootstrap-median.R 100 # Course Files script, B = 100
Rscript R/bootstrap-fast.R 100       # optimized R, same results
Rscript R/bootstrap-boot.R 1000      # boot package on the nine timing files
```

## Using bootmedian

`bootmedian` does two things:

```
bootmedian -file data/timing/normal-n10000.csv [options]   analyze one dataset
bootmedian -study data/study [options]                     rerun the R Monte Carlo study
```

| Option | Default | What it does |
|---|---|---|
| `-file` | | CSV file to analyze. It needs a column named `value` |
| `-study` | | Folder with the three study files from `R/generate-data.R` |
| `-B` | 1000 | Number of bootstrap samples (at least 2) |
| `-seed` | 9999 | Random seed, so results can be repeated |
| `-workers` | all cores | How many CPU cores to use. Results are the same either way |
| `-log` | screen | Save the progress log to a file |
| `-cpuprofile`, `-memprofile` | | Save profiling data, to see where time and memory go |

**Input:** a CSV file with a header row and a column named `value`. Other columns are ignored, so files saved from Excel work too.

**Output** for one dataset:

```
$ ./bootmedian -file data/timing/normal-n100000.csv -B 1000 -log run.log
Bootstrap estimate of the median for data/timing/normal-n100000.csv
  values:              100000
  bootstrap samples:   1000
  median:              100.008
  SE of the median:    0.0427
  95% percentile CI:   [99.925, 100.097]
  bootstrap time:      0.154 seconds
```

The study mode prints the same report as `R/bootstrap-fast.R`, so the two can be compared line by line.

**Bad input** gets a clear, one-line message:

```
$ ./bootmedian -file data/timing/normal-n10000.csv -B 1
error: -B must be at least 2, got 1
Run bootmedian -h to see the options.

$ ./bootmedian -file prices.csv
error: prices.csv line 3, column "value": "abc" is not a number

$ ./bootmedian -study empty-folder
error: open empty-folder/normal.csv: no such file or directory
Create the study files first with: Rscript R/generate-data.R
```

Run `bootmedian` with no options, or with `-h`, to see the help.

## Building the executable

| Command | What you get |
|---|---|
| `make build` or `go build -o bootmedian .` | `bootmedian` for your computer |
| `go build -o bootmedian.exe .` | `bootmedian.exe` on Windows |
| `make dist` | All five programs below, in `dist/` |

Go can build programs for other systems from any computer:

| File | Runs on |
|---|---|
| `dist/bootmedian-mac-arm64` | Mac with Apple Silicon (M1 to M4) |
| `dist/bootmedian-mac-intel` | Mac with Intel |
| `dist/bootmedian.exe` | Windows |
| `dist/bootmedian-linux-amd64` | Linux servers, such as AWS EC2 Intel instances |
| `dist/bootmedian-linux-arm64` | Linux on ARM, such as AWS Graviton instances |

Each one is a single 3 MB file with nothing else to install. Copy it and run it.

- **Mac:** the first time, macOS may block a downloaded program. Run `xattr -d com.apple.quarantine dist/bootmedian-mac-arm64` once, or right-click it in Finder and choose Open.
- **Windows:** it's a command-line program, so run it from PowerShell or Command Prompt: `.\bootmedian.exe -file data\timing\normal-n10000.csv`. Double-clicking only flashes the help text.
- A Mac `.app` is meant for programs with windows and buttons, so the Mac version is a plain program file too.

## Testing, benchmarking, profiling, and logging

| Command | What it does |
|---|---|
| `make test` | Runs all 37 tests |
| `make race` | Runs the tests while checking the multi-core code for memory clashes |
| `make cover` | Shows which lines the tests cover, in your browser |
| `make bench` | Measures speed and memory of the bootstrap package |
| `make profile` | Runs a big job and shows where the CPU time went |
| `make compare` | Times every R and Go version (about 7 minutes; needs R) |
| `make compare-table` | Rebuilds the comparison table from saved results, without rerunning |

What the tests check:

| Test file | Checks |
|---|---|
| `bootstrap/stats_test.go` | Mean, SD, and median match R. The fast median matches the simple one on 1,500 datasets, including repeated, sorted, and identical values |
| `bootstrap/bootstrap_test.go` | Bad input gives clear errors. Your data is never changed. The same seed gives the same answer. **1 or 100 cores give identical results.** Bootstrap answers match theory |
| `data_test.go` | Every kind of bad CSV file (empty, missing column, text, blanks, `NaN`, missing file) gets a clear message |
| `main_test.go`, `study_test.go` | Every bad option, full runs of both modes, and that logs and profiles get written |

Test data come from a random number generator with a fixed seed, so every test run is identical.

## What bootstrapping is

Bootstrapping answers a common question: **how sure can you be about a number, like a median, when you only have one sample?**

1. Draw n values from your sample **with replacement**. Some values get picked twice, some not at all.
2. Find the median of that new sample.
3. Repeat **B** times (for example, 1,000).
4. The standard deviation of those B medians is the **standard error** of the median.

For the mean, a simple formula gives the standard error (σ/√n). For the median, no simple formula works for every kind of data, so the bootstrap is useful. Every resample is independent, so it's easy to split the work across CPU cores.

**The data.** `R/generate-data.R` creates three populations. All have mean 100 and SD 10, so only the shape is different:

| Shape | How it's made | Median | True standard error of the median |
|---|---|---|---|
| Symmetric | Normal(100, 10) | 100 | about 12.53/√n |
| Right-skewed | 90 + Exponential(mean 10) | 96.93 | 10/√n |
| Left-skewed | 110 − Exponential(mean 10) | 103.07 | 10/√n |

We know the true answer for each shape, so we can check both R and Go against it. The study follows the Course Files script: 100 samples each of size 25, 100, 225, and 400.

## Finding R and Go packages

**R has well-known packages.** Searching [CRAN](https://cran.r-project.org/) and [r-universe](https://r-universe.dev/search/?q=bootstrap) turns up many. The two standard ones:

- **boot** (Davison and Hinkley 1997): https://cran.r-project.org/package=boot
- **bootstrap** (Efron and Tibshirani 1993): https://cran.r-project.org/package=bootstrap

I used `boot`. It comes with R, works with any statistic, and gives confidence intervals. One detail that matters for cost: `boot()` stores every resample's positions in memory at once, so its memory grows with B × n.

**Go has none.** [Awesome Go](https://awesome-go.com/#science-and-data-analysis) and [pkg.go.dev](https://pkg.go.dev/) have no established bootstrap package. The statistics libraries have building blocks only:

- [gonum/stat](https://pkg.go.dev/gonum.org/v1/gonum/stat): mean, SD, quantiles. No resampling.
- [montanaflynn/stats](https://pkg.go.dev/github.com/montanaflynn/stats): median and standard error of the mean. No resampling.
- [hamodikk/bootstrap_go](https://github.com/hamodikk/bootstrap_go): a student example whose author isn't sure it matches R.

So I wrote my own small Go package, `bootstrap`. Its only outside piece is the Mersenne Twister random number generator the assignment suggested ([seehuhn/mt19937](https://github.com/seehuhn/mt19937)). That's the 64-bit version, while R uses the 32-bit version, so R and Go draw different random numbers. Both read the same data files, so the results agree statistically, not digit for digit.

## How the Go version was built

R first, then Go one piece at a time, testing each piece before moving on:

1. **Data:** shared CSV files, checked against the target mean, SD, and shape.
2. **R starting point:** the Course Files script, reading the CSVs and covering three shapes. 16.8 s at B = 100.
3. **Faster R:** the Course Files script adds rows to a table with `rbind()` inside its loop, which copies the whole table every time. Setting up the result vectors in advance gave **the same output 3.5× faster**. This makes the comparison fair: Go has to beat good R, not slow R.
4. **R package:** `boot()` on the large datasets.
5. **Go statistics:** mean, SD, and median, each checked against R.
6. **Go bootstrap:** `Run(data, B, rng, statistics...)`, which works with any statistic, not just the median.
7. **The `bootmedian` program:** input checking, logging, profiling, and the R-style report.
8. **Speed fixes:** profiling, then three measured changes ([see below](#making-go-faster)).
9. **Comparison:** every R and Go version, run three times each.

Choices worth knowing about:

- **Any statistic works.** You pass in the function (median, mean, and so on), so the package isn't limited to medians.
- **Same answer on any number of cores.** The work is split into 64 fixed blocks, each with its own seed, so 1 core and 10 cores give identical results. A test checks this.
- **Your data is never changed**, even though the fast median shuffles values around internally.
- **Results and logs are kept separate.** Results go to the screen and logs to their own stream, so a saved results file never has log lines mixed in.

## Results

### Do R and Go agree?

**Yes.** Here is the standard error of the median from the Monte Carlo study (B = 1,000, averaged over 100 samples):

| n | Normal: R | Normal: Go | True | Right-skewed: R | Right-skewed: Go | Left-skewed: R | Left-skewed: Go | True (skewed) |
|---|---|---|---|---|---|---|---|---|
| 25 | 2.63 | 2.63 | 2.51 | 2.33 | 2.32 | 2.09 | 2.11 | 2.00 |
| 100 | 1.26 | 1.26 | 1.25 | 0.99 | 0.99 | 1.00 | 1.00 | 1.00 |
| 225 | 0.84 | 0.84 | 0.84 | 0.69 | 0.69 | 0.68 | 0.68 | 0.67 |
| 400 | 0.63 | 0.63 | 0.63 | 0.51 | 0.51 | 0.50 | 0.50 | 0.50 |

- R and Go match in 10 of 12 cells and are never more than 0.02 apart.
- Both match the true values from n = 100 up.
- At n = 25 both come out a little high. That's normal: with only 25 values, a resample can only produce a few different medians.

And for single large datasets (B = 1,000), compared with R's `boot` package:

| n | Median (both) | SE: R `boot` | SE: Go | True SE | 95% CI: R | 95% CI: Go |
|---|---|---|---|---|---|---|
| 1,000 | 100.259 | 0.331 | 0.343 | 0.396 | [99.64, 100.97] | [99.59, 101.05] |
| 10,000 | 99.991 | 0.118 | 0.123 | 0.125 | [99.76, 100.26] | [99.76, 100.26] |
| 100,000 | 100.008 | 0.041 | 0.043 | 0.040 | [99.93, 100.09] | [99.93, 100.10] |

The medians match exactly. The standard errors differ only by normal bootstrap noise, and both get closer to the truth as n grows.

Two more findings:

- **Shape matters.** For normal data, the median's standard error is about 25% bigger than the mean's. For skewed data, the two are about the same. So the median gives up less precision on skewed data.
- **The Course Files script measures something slightly different.** Its "SE Median from Bootstrap Samples" mostly measures how much medians vary *between* samples. The scripts here print that line too, for comparison, plus the standard bootstrap standard error shown above.

### Speed and memory

| Workload | Program | Runs | Wall time (s) | CPU time (s) | Peak memory (MB) | Faster than R | Less CPU than R | Less memory than R |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| Monte Carlo study, B=100 | **Go, 1 core** | 3 | 0.301 | 0.310 | 13 | 57.3× | 55.5× | 10.5× |
| Monte Carlo study, B=100 | Go, all cores | 3 | 0.074 | 0.385 | 15 | 234.3× | 44.7× | 9.3× |
| Monte Carlo study, B=100 | R optimized | 3 | 4.956 | 4.920 | 130 | 3.5× | 3.5× | 1.1× |
| Monte Carlo study, B=100 | R original | 3 | 17.270 | 17.224 | 139 | 1.0× | 1.0× | 1.0× |
| Monte Carlo study, B=1000 | **Go, 1 core** | 3 | 2.595 | 2.601 | 14 | 65.5× | 65.3× | 10.1× |
| Monte Carlo study, B=1000 | Go, all cores | 3 | 0.384 | 3.377 | 15 | 442.8× | 50.3× | 9.2× |
| Monte Carlo study, B=1000 | R optimized | 3 | 42.934 | 42.892 | 158 | 4.0× | 4.0× | 0.9× |
| Monte Carlo study, B=1000 | R original | 1 | 169.963 | 169.902 | 141 | 1.0× | 1.0× | 1.0× |
| one dataset, n=1000, B=1000 | **Go, 1 core** | 3 | 0.014 | 0.013 | 5 | 21.5× | 19.4× | 20.1× |
| one dataset, n=1000, B=1000 | Go, all cores | 3 | 0.004 | 0.016 | 7 | 66.4× | 16.3× | 16.7× |
| one dataset, n=1000, B=1000 | R boot package | 3 | 0.292 | 0.260 | 109 | 1.0× | 1.0× | 1.0× |
| one dataset, n=10000, B=1000 | **Go, 1 core** | 3 | 0.108 | 0.108 | 6 | 7.4× | 7.1× | 26.0× |
| one dataset, n=10000, B=1000 | Go, all cores | 3 | 0.019 | 0.139 | 8 | 43.2× | 5.5× | 20.6× |
| one dataset, n=10000, B=1000 | R boot package | 3 | 0.807 | 0.770 | 165 | 1.0× | 1.0× | 1.0× |
| one dataset, n=100000, B=1000 | **Go, 1 core** | 3 | 1.043 | 1.044 | 11 | 5.2× | 5.2× | 98.3× |
| one dataset, n=100000, B=1000 | Go, all cores | 3 | 0.161 | 1.393 | 16 | 34.0× | 3.9× | 66.1× |
| one dataset, n=100000, B=1000 | R boot package | 3 | 5.472 | 5.436 | 1075 | 1.0× | 1.0× | 1.0× |

How to read it:

- **Order:** in each workload, rows go from least to most CPU time. The best is in **bold**. The "× than R" columns compare each row with that workload's R version (R original, or the R boot package).
- **Wall time vs. CPU time:** wall time is how long you wait. CPU time is how much processor time was used, added up over all cores. Cloud servers charge for cores by the hour, so **CPU time is what drives cost.**
- **Small samples (the study):** Go is about 60× faster than the Course Files R script and 16× faster than optimized R. With small samples, most of R's time goes to the overhead of calling functions like `sample()` and `median()` over and over. Go turns into machine code, so it barely has that overhead.
- **Large samples (n = 100,000):** the gap shrinks to about 5×, because R's own sorting and sampling are already written in fast C code. Here Go wins mostly because of a better median method (quickselect).
- **Memory:** `boot()` used about 1 GB for the biggest job, because it stores all 100 million resample positions at once. Go used 5 to 16 MB. About 100 MB of every R number is just R itself starting up.
- **All cores:** using all 10 cores cuts Go's wait time 3.5× to 7× more, but uses slightly more total CPU, because the cores have to coordinate.

Measured on an Apple M4 laptop (10 cores) with Go 1.27.1 and R 4.5.1. Rerun with `make compare`.

### Making Go faster

The first Go version was a straight copy of the R logic. It was already 23× faster than the Course Files script on the study, but on the large dataset it was **no faster than R's `boot`**. Profiling showed why. Each fix was measured before and after (n = 100,000, B = 100):

| Step | What changed | Time | Memory | What the profile showed |
|---|---|---|---|---|
| 0 | Straight copy of R | 599 ms | 154 MB | 98% of the time was spent sorting. Nearly all memory went to copying the data twice per resample |
| 1 | Stop copying: reuse one buffer | 603 ms | **0.77 MB** | Memory dropped 99.5%, but speed didn't change. Copying wasn't the slow part |
| 2 | Find the median with quickselect instead of sorting everything | **102 ms** | 0.77 MB | You only need the middle value, not a full sort. 6× faster |
| 3 | Spread the resamples across all 10 cores | **17 ms** | 8.6 MB | Everything was running on 1 of 10 cores. Another 6× faster |

**Overall: 35× faster and 18× less memory than the first Go version.** All tests passed after every step. Benchmark files are in `docs/benchmarks/` and profiles in `docs/profiles/`.

## What I learned from testing, benchmarking, profiling, and logging

**Testing.** The best tests check rules that must always hold, not just one example: half the values are below the median; constant data has a standard error of 0; 1 core and 10 cores give the same answer; results match theory. When I swapped in quickselect, a test comparing it with the old median on 1,500 datasets showed the faster code was still right. Twice a "failing" test turned out to be a mistake in the test itself. One example: `(1 − 0.90) / 2` isn't exactly 0.05 on a computer, so tests on decimals need a small tolerance.

**Race detector.** `go test -race` watches the multi-core code for two cores writing to the same memory at once. It found no problems, because each core writes only to its own slots.

**Benchmarking.** `go test -bench` measures time and memory per run. Running each benchmark several times and comparing with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) shows whether a change actually helped or was just noise.

**Profiling changed the plan.** I expected copying data to be the slow part. The profile showed sorting took 98% of the time. Removing the copies saved memory but no time; only quickselect fixed the speed. **Without the profile, I would have fixed the wrong thing.**

**Logging.** Go's built-in `log/slog` writes timestamped lines like `msg="bootstrap finished" duration=166.621958ms`, showing how long each stage took and how much memory was used. Example logs are in `docs/logs/`.

**Measuring fairly took care.** R's timing tools only see inside R, so `cmd/compare` asks the operating system instead. That measures R and Go exactly the same way.

## Recommendation to the consultancy

### How much would Go save?

**Cloud provider: AWS EC2.** On-demand Linux prices in us-east-1, checked October 2026. Bigger sizes cost proportionally more (for example, `c7i.xlarge` costs twice `c7i.large`).

| Instance | vCPUs | Memory | Price per hour | Per vCPU-hour | Good for |
|---|---|---|---|---|---|
| c7g.large (Graviton, ARM) | 2 | 4 GiB | $0.0725 | $0.036 | Heavy computing, cheapest |
| c7i.large (Intel) | 2 | 4 GiB | $0.0892 | $0.045 | Heavy computing |
| m7i.large (Intel) | 2 | 8 GiB | $0.1008 | $0.050 | General use |
| r7i.large (Intel) | 2 | 16 GiB | $0.1323 | $0.066 | Jobs that need lots of memory |

**How the saving is figured:** if the firm keeps its servers busy, it pays for the CPU time its jobs use. So the saving is simply how much less CPU time Go needs. On a c7i.large:

| Job | Compared with | R CPU (s) | Go CPU (s) | R cost per 100,000 runs | Go cost per 100,000 runs | Saving |
|---|---|---:|---:|---:|---:|---:|
| Monte Carlo study, B=1000 | R original | 169.902 | 2.601 | $210.49 | $3.22 | **98%** |
| Monte Carlo study, B=1000 | R optimized | 42.892 | 2.601 | $53.14 | $3.22 | **94%** |
| Monte Carlo study, B=100 | R optimized | 4.920 | 0.310 | $6.10 | $0.38 | **94%** |
| One dataset, n=1,000, B=1000 | R boot | 0.260 | 0.013 | $0.32 | $0.02 | **95%** |
| One dataset, n=10,000, B=1000 | R boot | 0.770 | 0.108 | $0.95 | $0.13 | **86%** |
| One dataset, n=100,000, B=1000 | R boot | 5.436 | 1.044 | $6.73 | $1.29 | **81%** |

Each run is short, so costs are shown per 100,000 runs. Go runs one job per core here, which uses CPU more efficiently than splitting one job across cores.

**Memory saves even more.** R's `boot` memory grows with B × n: about 1 GB at n = 100,000 and B = 1,000, and about 10 GB at B = 10,000. Jobs that big need the expensive memory-heavy servers (r7i, $0.066 per vCPU-hour). Go stays under 20 MB, so it fits the cheapest servers (c7g Graviton, $0.036 per vCPU-hour). For a job like that, the total saving reaches about **90%**. That assumes the two server types run at similar speed per core; check it with one test run on each.

**Bottom line:** moving this kind of work from well-written R to Go cuts computing costs by about **80% to 95%**. Compared with R written like the Course Files script, it's about **98%**. For example, a firm spending $5,000 a month on this kind of computing would spend about $250 to $1,000. AWS discounts (Savings Plans, Spot) lower both bills by the same share, so the percentages still hold.

### So why keep R at all?

Because **computer time isn't the only cost. People's time costs money too.**

1. **Writing time.** The fast R version is about 70 lines. The Go version is about 1,300 lines with tests and took most of a day. For a one-off analysis, the analyst's hours cost far more than a few cents of cloud time.
2. **CRAN.** R has over 25,000 ready-made, tested packages. Go didn't even have a bootstrap package; I had to write one. For mixed models, survival analysis, or BCa confidence intervals, you'd have to build and check each one yourself in Go.
3. **Small jobs don't save real money.** 90% off a $20 monthly bill is $18. Not worth a rewrite.
4. **The firm's staff know R.** Switching means retraining people or hiring new ones.
5. **R checks Go.** I trusted the Go results partly *because* they matched R. Keeping R as the reference is how you catch mistakes in the Go version.

### What to do

| Use Go for | Keep R for |
|---|---|
| Big jobs that run again and again: simulations, Monte Carlo studies, daily pipelines | Exploring data and one-off analyses |
| Jobs with large n or B, especially ones that need lots of memory in R | Methods that depend on CRAN packages |
| Jobs that run on many cores or need to be installed easily (one file, no setup) | Small, rare jobs where savings are a few dollars |

**A practical path:** try out and check methods in R. When a method becomes a big, frequent job, move that job to Go, and use the R version to test the Go version on the same data, like this project does. Before rewriting anything, fix slow R habits like `rbind()` in a loop. That alone made the Course Files script about 4× faster, with no new language.

**Caveats:**

- All timings come from one Mac laptop. Times on AWS will differ, so confirm the ratios with one run on the target server (`make dist` builds the Linux programs).
- The savings cover computing only, not storage or data transfer.
- They don't include the cost of rewriting and testing code in Go.

## GenAI Tools

I used **Claude Code**, Anthropic's command-line coding assistant (model Claude Opus 5.5), in my terminal throughout this assignment, on November 10, 2026.

**How I used it:**
- **Writing and running code.** It wrote the R and Go code one file at a time, explaining each step. It ran the tests, benchmarks, profiles, and the R vs. Go comparison, and reported failures, including mistakes in its own tests, before fixing them.
- **Research.** It searched the web for current AWS EC2 prices and for existing Go bootstrap packages. The sources are linked in this README.

**How the results were checked:** every number in this README comes from the programs and output files in this repository and can be reproduced with `make test`, `make bench`, and `make compare`. The R and Go results were compared with each other and with statistical theory.

## References

Davison, Anthony C., and David V. Hinkley. 1997. *Bootstrap Methods and Their Application.* Cambridge University Press.

Efron, Bradley, and Robert J. Tibshirani. 1993. *An Introduction to the Bootstrap.* Chapman and Hall/CRC.

Gelman, Andrew, and Aki Vehtari. 2021. "What Are the Most Important Statistical Ideas of the Past 50 Years?" *Journal of the American Statistical Association* 116: 2087–2097. https://arxiv.org/abs/2012.00174

Hoare, C. A. R. 1961. "Algorithm 65: Find." *Communications of the ACM* 4 (7): 321–322.

McConnell, Steve. 2004. *Code Complete.* 2nd ed. Microsoft Press.

Miller, Thomas W. 2015. *Modeling Techniques in Predictive Analytics with Python and R.* Pearson Education.

Wirth, Niklaus. 1976. *Algorithms + Data Structures = Programs.* Prentice-Hall.

AWS EC2 prices: [cloud-credits.org](https://cloud-credits.org/prices/ec2), [usage.ai](https://www.usage.ai/blogs/aws/ec2/pricing/), and [costgoat.com](https://costgoat.com/pricing/amazon-ec2), which agree with one another. Check the [AWS EC2 On-Demand pricing page](https://aws.amazon.com/ec2/pricing/on-demand/) for current prices.
