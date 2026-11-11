# Creates the shared datasets so R and Go analyze identical numbers.
# Run from the project folder: Rscript R/generate-data.R

POP_MEAN <- 100
POP_SD <- 10
SHAPES <- c("normal", "right-skewed", "left-skewed")

N_ITERATIONS <- 100
STUDY_SAMPLE_SIZES <- c(25, 100, 225, 400)
TIMING_SAMPLE_SIZES <- c(1000, 10000, 100000)

# Round to 6 decimals so R and Go read exactly the same numbers.
DECIMAL_PLACES <- 6

# Name R's default generator (Mersenne Twister) explicitly.
RNGkind(kind = "Mersenne-Twister", normal.kind = "Inversion", sample.kind = "Rejection")
set.seed(9999)  # same seed as the Course Files script

# One sample: normal, or an exponential shifted to mean 100 and SD 10 (mirrored for left-skewed).
draw_sample <- function(shape, n) {
  values <- switch(shape,
    "normal"       = rnorm(n, mean = POP_MEAN, sd = POP_SD),
    "right-skewed" = (POP_MEAN - POP_SD) + rexp(n, rate = 1 / POP_SD),
    "left-skewed"  = (POP_MEAN + POP_SD) - rexp(n, rate = 1 / POP_SD),
    stop("unknown shape: ", shape)
  )
  round(values, DECIMAL_PLACES)
}

# All study samples, joined with one rbind() at the end (rbind() inside a loop is slow).
make_study_data <- function(shape) {
  samples <- list()
  for (iteration in seq_len(N_ITERATIONS)) {
    for (n in STUDY_SAMPLE_SIZES) {
      samples[[length(samples) + 1]] <-
        data.frame(iteration = iteration, n = n, value = draw_sample(shape, n))
    }
  }
  do.call(rbind, samples)
}

write_data <- function(data, path) {
  write.csv(data, path, row.names = FALSE, quote = FALSE)
  cat(sprintf("  wrote %-38s %7d rows\n", path, nrow(data)))
}

# Skewness: about 0 symmetric, +2 right-skewed, -2 left-skewed.
skewness <- function(x) mean((x - mean(x))^3) / sd(x)^3

if (!dir.exists("data")) {
  stop("run this script from the project root folder (the one containing data/)")
}
dir.create("data/study", showWarnings = FALSE)
dir.create("data/timing", showWarnings = FALSE)

# Fixed order from one seed, so reruns give identical files.
cat("Writing datasets\n")
for (shape in SHAPES) {
  write_data(make_study_data(shape), sprintf("data/study/%s.csv", shape))
  for (n in TIMING_SAMPLE_SIZES) {
    write_data(data.frame(value = draw_sample(shape, n)),
               sprintf("data/timing/%s-n%d.csv", shape, n))
  }
}

# Check the largest files against the target mean and SD.
cat("\nCheck of the n = 100000 timing files (target: mean 100, SD 10)\n")
for (shape in SHAPES) {
  values <- read.csv(sprintf("data/timing/%s-n100000.csv", shape))$value
  cat(sprintf("  %-13s mean %6.2f   SD %5.2f   median %6.2f   skewness %5.2f\n",
              shape, mean(values), sd(values), median(values), skewness(values)))
}
