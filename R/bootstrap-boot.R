# SE of the median with the CRAN boot package: https://cran.r-project.org/package=boot
# Run from the project folder: Rscript R/bootstrap-boot.R [B] [one CSV file]

library(boot)

POP_SD <- 10
SHAPES <- c("normal", "right-skewed", "left-skewed")
TIMING_SAMPLE_SIZES <- c(1000, 10000, 100000)

# Theoretical SE of the median: SD * sqrt(pi/2) / sqrt(n) if normal, SD / sqrt(n) if skewed.
theoretical_se_median <- function(shape, n) {
  if (shape == "normal") POP_SD * sqrt(pi / 2) / sqrt(n) else POP_SD / sqrt(n)
}

# Theory is known only for our synthetic files (NA for others).
theoretical_se_for_file <- function(path, n) {
  shape <- sub("-n[0-9]+\\.csv$", "", basename(path))
  if (shape %in% SHAPES) theoretical_se_median(shape, n) else NA
}

# boot() calls this for each bootstrap sample, with the positions it drew.
median_of_resample <- function(data, indices) median(data[indices])

args <- commandArgs(trailingOnly = TRUE)
B <- if (length(args) >= 1) suppressWarnings(as.integer(args[1])) else 1000
if (is.na(B) || B < 2) stop("B must be a whole number of at least 2")

data_files <- if (length(args) >= 2) {
  args[2]
} else {
  sprintf("data/timing/%s-n%d.csv",
          rep(SHAPES, each = length(TIMING_SAMPLE_SIZES)),
          rep(TIMING_SAMPLE_SIZES, times = length(SHAPES)))
}

cat("\nBootstrap SE of the median with boot::boot(), B =", B, "\n")
cat("Note: boot() stores all B x n resample positions at once,",
    "so its memory use grows with B times n.\n\n")
cat(sprintf("%-20s %7s %8s %9s %9s %21s %8s\n",
            "dataset", "n", "median", "SE boot", "SE theory",
            "95% percentile CI", "seconds"))

for (path in data_files) {
  if (!file.exists(path)) stop("file not found: ", path)
  values <- read.csv(path)$value
  n <- length(values)
  set.seed(9999)
  run_time <- system.time(
    boot_result <- boot(values, statistic = median_of_resample, R = B)
  )[["elapsed"]]
  # t0 = median of the data; t = the B bootstrap medians
  interval <- boot.ci(boot_result, type = "perc")$percent[4:5]
  cat(sprintf("%-20s %7d %8.3f %9.4f %9.4f   [%8.3f, %8.3f] %8.2f\n",
              sub("\\.csv$", "", basename(path)), n, boot_result$t0,
              sd(boot_result$t), theoretical_se_for_file(path, n),
              interval[1], interval[2], run_time))
}
cat("\n----- Run Complete -----\n")
