package logger

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

// LogLevel defines the logging level
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var (
	// Use atomic for lock-free thread-safe access to log level
	// This is more efficient than RWMutex for simple integer reads/writes
	currentLevel atomic.Int32
	infoColor    = color.New(color.FgCyan)
	warnColor    = color.New(color.FgYellow)
	errorColor   = color.New(color.FgRed)
	debugColor   = color.New(color.FgMagenta)
	successColor = color.New(color.FgGreen)
)

func init() {
	// Initialize atomic value with InfoLevel
	currentLevel.Store(int32(InfoLevel))
}

// SetLevel sets the current log level
func SetLevel(level LogLevel) {
	currentLevel.Store(int32(level))
}

// IsDebugEnabled reports whether debug logging is active.
func IsDebugEnabled() bool {
	return getLogLevel() <= DebugLevel
}

// SetVerbose enables debug logging
func SetVerbose(verbose bool) {
	if verbose {
		SetLevel(DebugLevel)
	}
}

// SetQuiet disables info and debug logging
func SetQuiet(quiet bool) {
	if quiet {
		SetLevel(WarnLevel)
	}
}

// getLogLevel safely reads the current log level (lock-free)
func getLogLevel() LogLevel {
	return LogLevel(currentLevel.Load())
}

// formatMessage formats a log message with timestamp
func formatMessage(level string, message string) string {
	timestamp := time.Now().Format("02/Jan/2006:15:04:05 -0700")
	return fmt.Sprintf("[%s] %s", timestamp, message)
}

// Debug logs a debug message (only shown in verbose mode)
func Debug(message string) {
	if getLogLevel() <= DebugLevel {
		_, _ = debugColor.Println("🐛", formatMessage("DEBUG", message))
	}
}

// Info logs an informational message
func Info(message string) {
	if getLogLevel() <= InfoLevel {
		_, _ = infoColor.Println("ℹ", formatMessage("INFO", message))
	}
}

// Warn logs a warning message
func Warn(message string) {
	if getLogLevel() <= WarnLevel {
		_, _ = warnColor.Println("⚠", formatMessage("WARN", message))
	}
}

// Error logs an error message
func Error(message string) {
	_, _ = errorColor.Println("✖", formatMessage("ERROR", message))
}

// Success logs a success message
func Success(message string) {
	if getLogLevel() <= InfoLevel {
		_, _ = successColor.Println("✓", formatMessage("SUCCESS", message))
	}
}

// Fatal logs an error message and exits
func Fatal(message string) {
	_, _ = errorColor.Println("✖", formatMessage("FATAL", message))
	os.Exit(1)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	Debug(fmt.Sprintf(format, args...))
}

// Infof logs a formatted informational message
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	Warn(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Successf logs a formatted success message
func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

// Fatalf logs a formatted error message and exits
func Fatalf(format string, args ...interface{}) {
	Fatal(fmt.Sprintf(format, args...))
}
