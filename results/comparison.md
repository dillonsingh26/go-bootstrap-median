# R vs. Go comparison

Machine: Apple M4, darwin/arm64, 10 cores. go1.27.1; R version 4.5.1 (2025-06-13).

Medians of 3 runs (slow cases run once). Times include starting the program and reading the data. CPU time is summed over all cores. Within each workload, rows run from least to most CPU time; the best is in bold. The last three columns compare each row with the workload's R reference (R original or R boot package).

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
