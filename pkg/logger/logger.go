package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	logger *zap.Logger
	sugar  *zap.SugaredLogger
}

// Config holds the logger configuration
type Config struct {
	Level string
	File  string
}

// NewLogger creates a new logger with default configuration
func NewLogger() *Logger {
	hostname, _ := os.Hostname()
	// Get the current date in the format YYYY-MM-DD
	currentDate := time.Now().Format("2006-01-02")
	// Construct the log file path with the current date
	LogFilePath := filepath.Join("./var/logs/", fmt.Sprintf("%s_%s.log", hostname, currentDate))
	return NewLoggerWithConfig(Config{
		Level: "info",
		File:  LogFilePath,
	})
}

// NewLoggerWithConfig creates a new logger with the specified configuration
func NewLoggerWithConfig(config Config) *Logger {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(config.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	// Configure encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Configure output
	var cores []zapcore.Core

	// Console output
	consoleEncoder := zapcore.NewJSONEncoder(encoderConfig)
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)
	cores = append(cores, consoleCore)

	// File output if configured
	if config.File != "" {
		file, err := os.OpenFile(config.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
			fileCore := zapcore.NewCore(
				fileEncoder,
				zapcore.AddSync(file),
				level,
			)
			cores = append(cores, fileCore)
		}
	}

	// Create logger
	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{
		logger: logger,
		sugar:  logger.Sugar(),
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	if len(fields) > 0 {
		l.logger.Debug(msg, fieldsToZapFields(fields[0])...)
	} else {
		l.logger.Debug(msg)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	if len(fields) > 0 {
		l.logger.Info(msg, fieldsToZapFields(fields[0])...)
	} else {
		l.logger.Info(msg)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	if len(fields) > 0 {
		l.logger.Warn(msg, fieldsToZapFields(fields[0])...)
	} else {
		l.logger.Warn(msg)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, fields ...map[string]interface{}) {
	zapFields := []zap.Field{}
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	if len(fields) > 0 {
		zapFields = append(zapFields, fieldsToZapFields(fields[0])...)
	}
	l.logger.Error(msg, zapFields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, err error, fields ...map[string]interface{}) {
	zapFields := []zap.Field{}
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	if len(fields) > 0 {
		zapFields = append(zapFields, fieldsToZapFields(fields[0])...)
	}
	l.logger.Fatal(msg, zapFields...)
}

// WithField returns a new logger with a field added to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		logger: l.logger.With(zap.Any(key, value)),
		sugar:  l.logger.With(zap.Any(key, value)).Sugar(),
	}
}

// WithFields returns a new logger with multiple fields added to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	return &Logger{
		logger: l.logger.With(fieldsToZapFields(fields)...),
		sugar:  l.logger.With(fieldsToZapFields(fields)...).Sugar(),
	}
}

// WithRequestID returns a new logger with a request ID field
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.WithField("request_id", requestID)
}

// WithTimestamp returns a new logger with a timestamp field
func (l *Logger) WithTimestamp() *Logger {
	// Zap already includes timestamp by default, so this is a no-op
	return l
}

// WithTime returns a new logger with a time field
func (l *Logger) WithTime(t time.Time) *Logger {
	return &Logger{
		logger: l.logger.With(zap.Time("time", t)),
		sugar:  l.logger.With(zap.Time("time", t)).Sugar(),
	}
}

// Helper function to convert map[string]interface{} to []zap.Field
func fieldsToZapFields(fields map[string]interface{}) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return zapFields
}
