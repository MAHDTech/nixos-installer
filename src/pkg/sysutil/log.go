// Package sysutil provides utility functions for system operations.
// This file contains the logging functionality for the sysutil package.
package sysutil

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

// Log Levels.
const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Color returns the ANSI color code for the log level
func (l LogLevel) Color() string {
	switch l {
	case DEBUG:
		return "\033[33m" // Yellow
	case INFO:
		return "\033[34m" // Blue
	case WARN:
		return "\033[38;5;208m" // Orange
	case ERROR:
		return "\033[31m" // Red
	default:
		return "\033[0m" // Reset
	}
}

// Logger provides structured logging with file and console output
type Logger struct {
	fileLogger   *log.Logger
	consoleLevel LogLevel
	fileLevel    LogLevel
	file         *os.File
}

var (
	// Global logger instance
	globalLogger *Logger
	// Color reset code
	colorReset = "\033[0m"
)

// InitLogger initializes the global logger with file and console output
func InitLogger(logFile string, consoleLevel, fileLevel LogLevel) error {
	// Create log directory if it doesn't exist
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	// #nosec G304 -- This is a legitimate use case for opening a log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer for file logging
	fileWriter := io.MultiWriter(file)

	// Create loggers
	fileLogger := log.New(fileWriter, "", log.LstdFlags)

	globalLogger = &Logger{
		fileLogger:   fileLogger,
		consoleLevel: consoleLevel,
		fileLevel:    fileLevel,
		file:         file,
	}

	return nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log writes a message to both file and console based on log levels
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// File logging
	if level >= l.fileLevel {
		l.fileLogger.Printf("[%s] %s: %s", timestamp, level.String(), message)
	}

	// Console logging
	if level >= l.consoleLevel {
		coloredLevel := level.Color() + level.String() + colorReset
		fmt.Printf("[%s] %s: %s\n", timestamp, coloredLevel, message)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Debug global convenience function
func Debug(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(format, args...)
	}
}

// Info global convenience function
func Info(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(format, args...)
	}
}

// Warn global convenience function
func Warn(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(format, args...)
	}
}

// Error global convenience function
func Error(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, args...)
	}
}

// Close closes the global logger
func Close() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

// Progress represents a progress indicator
type Progress struct {
	title     string
	current   int
	total     int
	startTime time.Time
}

// NewProgress creates a new progress indicator
func NewProgress(title string, total int) *Progress {
	return &Progress{
		title:     title,
		total:     total,
		startTime: time.Now(),
	}
}

// Update updates the progress and displays it
func (p *Progress) Update(current int) {
	p.current = current
	p.display()
}

// Increment increments the progress by 1
func (p *Progress) Increment() {
	p.current++
	p.display()
}

// display shows the current progress with a progress bar
func (p *Progress) display() {
	percentage := float64(p.current) / float64(p.total) * 100
	barWidth := 30
	filled := int(float64(barWidth) * percentage / 100)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	elapsed := time.Since(p.startTime)
	eta := time.Duration(0)
	if p.current > 0 {
		eta = time.Duration(float64(elapsed) * float64(p.total-p.current) / float64(p.current))
	}

	// Clear line and move cursor to beginning
	fmt.Print("\r\033[K")
	fmt.Printf("\033[34m%s\033[0m [%s] %d/%d (%.1f%%) %s/%s",
		p.title, bar, p.current, p.total, percentage,
		elapsed.Round(time.Second), eta.Round(time.Second))
}

// Complete marks the progress as complete
func (p *Progress) Complete() {
	p.current = p.total
	p.display()
	fmt.Println() // New line after progress
}

// Success prints a success message with green color
func Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("\033[32m✓ %s\033[0m\n", message)
}

// Failure prints a failure message with red color
func Failure(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("\033[31m✗ %s\033[0m\n", message)
}

// Section prints a section header with blue color
func Section(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("\n\n\033[34m==============================\033[0m\n")
	fmt.Printf("\033[34m=== %s ===\033[0m\n", message)
	fmt.Printf("\033[34m==============================\033[0m\n\n")
}

// SubSection prints a subsection header with cyan color
func SubSection(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("\n\n\033[36m----------------------------------\033[0m\n")
	fmt.Printf("\033[36m--- %s ---\033[0m\n", message)
	fmt.Printf("\033[36m----------------------------------\033[0m\n\n")
}
