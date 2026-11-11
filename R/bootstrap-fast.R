# The Course Files study rewritten for speed: no rbind() in loops, same results.
# Run from the project folder: Rscript R/bootstrap-fast.R [B]

POP_MEAN <- 100
POP_SD <- 10
SHAPES <- c("normal", "right-skewed", "left-skewed")
N_ITERATIONS <- 100
STUDY_SAMPLE_SIZES <- c(25, 100, 225, 400)

# B bootstrap means and medians of x, stored in vectors sized up front.
bootstrap_replicates <- function(x, B) {
  n <- length(x)
  boot_means <- numeric(B)
  boot_medians <- numeric(B)
  for (b in seq_len(B)) {
    resample <- sample(x, n, replace = TRUE)
    boot_means[b] <- mean(resample)
    boot_medians[b] <- median(resample)
  }
  list(means = boot_means, medians = boot_medians)
}

# Runs the study for one shape: one row per sample.
run_study <- function(samples, B) {
  n_rows <- N_ITERATIONS * length(STUDY_SAMPLE_SIZES)
  results <- data.frame(
    n = numeric(n_rows),
    sample_mean = numeric(n_rows),
    boot_mean = numeric(n_rows),        # average of the B bootstrap means
    boot_median = numeric(n_rows),      # average of the B bootstrap medians
    boot_se_mean = numeric(n_rows),     # SD of the B bootstrap means
    boot_se_median = numeric(n_rows)    # SD of the B bootstrap medians
  )
  row <- 0
  for (iteration in seq_len(N_ITERATIONS)) {
    for (n in STUDY_SAMPLE_SIZES) {
      row <- row + 1
      this_sample <- samples[[paste(iteration, n)]]
      replicates <- bootstrap_replicates(this_sample, B)
      results[row, ] <- c(n, mean(this_sample),
                          mean(replicates$means), mean(replicates$medians),
                          sd(replicates$means), sd(replicates$medians))
    }
  }
  results
}

# Prints the original report lines plus the textbook bootstrap SEs.
report_study <- function(results, shape, B) {
  cat("\nEstimated standard errors for the ", shape, " population using ",
      B, " bootstrap samples\n", sep = "")
  for (n in STUDY_SAMPLE_SIZES) {
    rows <- results[results$n == n, ]
    cat("\nSamples of size n =", n)
    cat("\n  SE Mean from Central Limit Theorem:", round(POP_SD / sqrt(n), 2))
    cat("\n  SE Mean from Samples:", round(sd(rows$sample_mean), 2))
    cat("\n  SE Mean from Bootstrap Samples:", round(sd(rows$boot_mean), 2))
    cat("\n  SE Median from Bootstrap Samples:", round(sd(rows$boot_median), 2))
    cat("\n  SE Mean, standard bootstrap:", round(mean(rows$boot_se_mean), 2))
    cat("\n  SE Median, standard bootstrap:", round(mean(rows$boot_se_median), 2))
    cat("\n")
  }
}

# Reads one shape's study file into samples keyed by "iteration n".
read_study_samples <- function(shape) {
  study_data <- read.csv(file.path("data", "study", paste0(shape, ".csv")))
  split(study_data$value, paste(study_data$iteration, study_data$n))
}

args <- commandArgs(trailingOnly = TRUE)
B <- if (length(args) >= 1) suppressWarnings(as.integer(args[1])) else 100
if (is.na(B) || B < 2) stop("B must be a whole number of at least 2")

cat("\nRunning study with", B, "bootstrap samples\n")
cat("\nStudy conditions:")
cat("\n  Population mean:", POP_MEAN, "SD:", POP_SD, "\n")

start_time <- proc.time()
for (shape in SHAPES) {
  set.seed(9999)  # same seed, reset per shape, as in run-bootstrap-median.R
  results <- run_study(read_study_samples(shape), B)
  report_study(results, shape, B)
}
elapsed <- (proc.time() - start_time)[["elapsed"]]
cat("\nElapsed time for all three shapes:", round(elapsed, 2), "seconds\n")
cat("\n----- Run Complete -----\n")
