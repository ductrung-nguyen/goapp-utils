package logger

import (
	"fmt"
	"os"
	"path"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/kardianos/osext"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Root logr.Logger

// LoggerConfig contains all configuration
type LoggerConfig struct {
	// The logging environment, "prod" for production and any other value for "development"
	Environment string `yaml:"environment"`
	// The encoder for logs, "json" and any other value for "console"
	Encoder string `yaml:"encoder"`
	// Folder is the log folder
	Folder string `yaml:"folder"`
	// Filename is the name of the log file
	Filename string `yaml:"filename"`
	// Should be also write the log to the console
	LogToConsole bool `yaml:"logToConsole"`
	// the bigger, the less important logs to user (for example, INFO=0, DEBUG=1,...)
	// if the level set to K, the app outputs only the messages are logged with level <= K
	Level int `yaml:"level"`
	// max size of each log file before rolling
	MaxSizeInMB int `yaml:"maxSizeInMB"`
	// number of backups
	MaxBackups int `yaml:"maxBackups"`
	// compress the log file
	Compress bool `yaml:"compress"`
	// max age of a log file
	MaxAge int `yaml:"maxAge"`
	// should we skip logging the caller and the line number
	SkipCaller bool `yaml:"skipCaller"`
}

func init() {
	InitLogger(nil)
}

// InitLogger initializes the logger based on running mode
func InitLogger(cf *LoggerConfig) {
	if cf == nil {
		cf = &LoggerConfig{
			Folder:       "logs",
			Filename:     "prod.log",
			LogToConsole: true,
			Level:        int(zap.DebugLevel),
			// max size of each log file before rolling
			MaxSizeInMB: 500,
			// number of backups
			MaxBackups: 2,
			// compress the log file
			Compress: true,
			// 90 days
			MaxAge: 90,
		}
	}

	Root = zapr.NewLogger(NewLogger(*cf))
}

func NewLogger(cf LoggerConfig) *zap.Logger {
	writerSyncer := getLogWriter(cf)
	encoder := getEncoder(cf.Environment, cf.Encoder)
	// Zap uses semantically named levels for logging (DebugLevel, InfoLevel, WarningLevel, ...).
	// Logr uses arbitrary numeric levels. By default logr's V(0) is zap's InfoLevel and V(1) is zap's DebugLevel (which is numerically -1).
	// Zap does not have named levels that are more verbose than DebugLevel
	// cf.Level == 2  means that log.V(<2).Info() calls will be active. 3 would enable log.V(<3).Info(), etc
	// setting the zap level to -128 (cf.Level = 128) really means "activate all logs"
	core := zapcore.NewCore(encoder, writerSyncer, zapcore.Level(-cf.Level))
	var logger *zap.Logger
	if cf.SkipCaller {
		logger = zap.New(core)
	} else {
		logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0))
	}

	return logger
}

// getEncoder returns a JSON encoder for the log
func getEncoder(env string, encoder string) zapcore.Encoder {
	var encoderConfig zapcore.EncoderConfig

	if env == "prod" {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	if encoder == "json" {
		return zapcore.NewJSONEncoder(encoderConfig)
	} else {
		return zapcore.NewConsoleEncoder(encoderConfig)
	}

}

// getLogWriter returns the log writer
func getLogWriter(cf LoggerConfig) zapcore.WriteSyncer {
	currentFolder, _ := osext.ExecutableFolder()
	fullFilename := path.Join(currentFolder, cf.Folder, cf.Filename)
	writeSyncers := []zapcore.WriteSyncer{}
	err := os.MkdirAll(cf.Folder, os.ModePerm)
	if err != nil {
		fmt.Printf("Error when creating log folder: %v\n", err)
	} else {
		writeSyncers = append(writeSyncers, zapcore.AddSync(&lumberjack.Logger{
			Filename:   fullFilename,
			MaxSize:    cf.MaxSizeInMB, // megabytes
			MaxBackups: cf.MaxBackups,
			MaxAge:     cf.MaxAge,   //days
			Compress:   cf.Compress, // disabled by default
		}))
	}

	if cf.LogToConsole {
		writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stdout))
	}

	return zapcore.NewMultiWriteSyncer(writeSyncers...)
}
