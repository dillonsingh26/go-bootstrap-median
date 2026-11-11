# example of bootstrap sampling and estimation of standard errors

# revised May 23, 2024 by Thomas W. Miller

# CHANGED: minimal edits for this project, each marked CHANGED (slow rbind() kept on purpose).
# CHANGED: run from the project folder: Rscript R/run-bootstrap-median.R [B]

# estimating standard error of the median,
# providing comparisons with the standard error of the mean

# the standard error is the standard deviation
# of the sampling distribution

# CHANGED: reads samples from data/study/ (3 shapes) instead of rnorm()

# of course, one of the advantages of bootstrap
# estimation of standard errors is that it may be
# used for any population distribution

# bootstrap sampling is sampling with replacement
# which can be accomplished with the R sample() function

# larger numbers of bootstrap samples B
# will yield better estimates of the true standard errors
# vary B by using separate executions of this program
# CHANGED: B may be given on the command line instead of editing the file
args = commandArgs(trailingOnly = TRUE)
B = if (length(args) >= 1) suppressWarnings(as.integer(args[1])) else 100
if (is.na(B) || B < 2) stop("B must be a whole number of at least 2")
cat("\nRunning study with",B,"bootstrap samples\n")

ROUTE_OUTPUT_TO_LISTING = FALSE
if (ROUTE_OUTPUT_TO_LISTING) {
    listing_file_name = paste("listing-from-",as.character(B),"-run-bootstrap-median.txt",sep="")
    cat("\nStudy results will be provided in:",listing_file_name,"\n")
    sink(listing_file_name)
}

# set the mean and standard deviation of the population distributions
popMean = 100
popSD = 10
studySampleSizes = c(25,100,225,400)
# CHANGED: the three population shapes written by R/generate-data.R
studyShapes = c("normal","right-skewed","left-skewed")

cat("\nStudy conditions:")
cat("\n  Population mean:",popMean,"SD:",popSD,"\n")

# the Central Limit Theorem states that the standard
# error of the mean should be equal to the population
# standard deviation divided by the square-root
# of the sample size

# CHANGED: time the whole study for the R vs. Go comparison
studyStartTime = proc.time()

# CHANGED: repeat the original study once for each population shape
for (shape in studyShapes) {

# CHANGED: read this shape's samples, keyed by "iteration n"
studyData = read.csv(file.path("data","study",paste(shape,".csv",sep="")))
studySamples = split(studyData$value, paste(studyData$iteration, studyData$n))

# set up a data frame for saving study results
studyResults = NULL

# CHANGED: seed now drives only sample(); reset for each shape
set.seed(9999) # for reproducible results for each study
for (iteration in 1:100) {
    # vary the sample size N
    for (nindex in seq(along=studySampleSizes)) {
        n = studySampleSizes[nindex]
        # CHANGED: was thisSample = rnorm(n, mean = popMean, sd = popSD)
        thisSample = studySamples[[paste(iteration, n)]]
        thisSampleMean = mean(thisSample)
        bootstrapSampleResults = NULL
        for (b in 1:B) {
            thisBootstrapSample = sample(thisSample, n, replace=TRUE)
            thisBootstrapMean = mean(thisBootstrapSample)
            thisBootstrapMedian = median(thisBootstrapSample)
            thisBootstrapSampleResults = data.frame(bootMean=thisBootstrapMean, bootMedian=thisBootstrapMedian)
            bootstrapSampleResults = rbind(thisBootstrapSampleResults,bootstrapSampleResults)
        }
        bootMean = mean(bootstrapSampleResults$bootMean)
        bootMedian = mean(bootstrapSampleResults$bootMedian)

        thisIterationResults=data.frame(n,
                                        sampleMean = thisSampleMean,
                                        bootMean,
                                        bootMedian)

        studyResults=rbind(studyResults, thisIterationResults)
    }
}

# bootstrap-estimated standard errors for each sample size n
# CHANGED: the heading names the population shape
cat("\nEstimated standard errors for the ",shape," population using ",B," bootstrap samples\n",sep="")
for (nindex in seq(along=studySampleSizes)) {
    n = studySampleSizes[nindex]
    nStudyResults = studyResults[(studyResults$n == n),]
    cat("\nSamples of size n =",n)
    cat("\n  SE Mean from Central Limit Theorem:", round(popSD/sqrt(n),digits=2))
    cat("\n  SE Mean from Samples:",round(sd(nStudyResults$sampleMean),digits=2))
    cat("\n  SE Mean from Bootstrap Samples:",round(sd(nStudyResults$bootMean),digits=2))
    cat("\n  SE Median from Bootstrap Samples:",round(sd(nStudyResults$bootMedian),digits=2))
    cat("\n")
}

} # CHANGED: end of the loop over population shapes

# CHANGED: report the elapsed (wall clock) time for the whole study
studyElapsed = (proc.time() - studyStartTime)[["elapsed"]]
cat("\nElapsed time for all three shapes:", round(studyElapsed, digits=2), "seconds\n")

if (ROUTE_OUTPUT_TO_LISTING)
   sink()

cat("\n----- Run Complete -----\n")
