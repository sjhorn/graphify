library(dplyr)
library(ggplot2)

analyze_data <- function(df) {
  result <- df %>%
    filter(value > 0) %>%
    group_by(category) %>%
    summarize(mean = mean(value))
  return(result)
}

plot_results <- function(data) {
  ggplot(data, aes(x = category, y = mean)) +
    geom_bar(stat = "identity")
}

DataProcessor <- function(config) {
  self <- list(
    config = config,
    data = NULL
  )
  class(self) <- "DataProcessor"

  self$load <- function(file) {
    self$data <- read.csv(file)
    return(self)
  }

  self$process <- function() {
    analyze_data(self$data)
  }

  return(self)
}

processor <- DataProcessor(list(verbose = TRUE))
processed <- processor$process()
