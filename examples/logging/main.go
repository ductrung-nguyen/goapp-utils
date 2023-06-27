package main

import (
	"errors"

	"rndwww.nce.amadeus.net/git/SPLUNK/goapp-utils/pkg/logger"
)

func main() {
	// Using logger with default configuration
	logger.Root.WithName("OptionalLoggerName").Info("This is a simple log at level 0")

	logger.Root.Info("This is a simple log at level 0 with some key-value pairs", "podName", "indexer-0", "No.", 1)
	logger.Root.WithValues("podName", "indexer-0", "No.", 1).Info("The same simple log at level 0 with some key-value pairs")
	logger.Root.Info("This is a simple log at level 1")
	logger.Root.V(2).Info("This message is not printed because its level =2, higher than the default max allowed level = 1")

	// if we want to change the configuration
	logger.InitLogger(&logger.LoggerConfig{
		Folder:       "logs", // where to store the log files
		Environment:  "prod", // or any other value to use Environment "development"
		Encoder:      "json", // or any other value to use encoder "console"
		LogToConsole: true,   // it will write logs to file and stdout
		Level:        3,      // The maximum level of the logs that can be printed
		MaxSizeInMB:  100,    // max size of each log file before rolling
		MaxAge:       10,     // max age of a log file
		Compress:     true,
	})

	logger.Root.V(2).Info("This message is printed as its level = 2, lower than max allowed level = 3")
	logger.Root.V(4).Info("This message is not printed anywhere as its level = 4, higher than max allowed level = 3")
	logger.Root.V(5).Error(errors.New("a dummy error"), "This error message is still printed even if its level is higher than the max allowed level")
}
