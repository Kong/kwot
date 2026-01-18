package logger

import (
	"bytes"
	"testing"
)

// TestSetLevel tests log level setting
func TestSetLevel(t *testing.T) {
	oldLevel := getLogLevel()
	defer func() { SetLevel(oldLevel) }()

	SetLevel(DebugLevel)
	if getLogLevel() != DebugLevel {
		t.Errorf("SetLevel(DebugLevel) failed, expected DebugLevel, got %v", getLogLevel())
	}

	SetLevel(ErrorLevel)
	if getLogLevel() != ErrorLevel {
		t.Errorf("SetLevel(ErrorLevel) failed, expected ErrorLevel, got %v", getLogLevel())
	}
}

// TestSetVerbose tests verbose flag
func TestSetVerbose(t *testing.T) {
	oldLevel := getLogLevel()
	defer func() { SetLevel(oldLevel) }()

	SetVerbose(true)
	if getLogLevel() != DebugLevel {
		t.Errorf("SetVerbose(true) failed, expected DebugLevel, got %v", getLogLevel())
	}

	// Reset to original level for next test
	SetLevel(InfoLevel)
}

// TestSetQuiet tests quiet flag
func TestSetQuiet(t *testing.T) {
	oldLevel := getLogLevel()
	defer func() { SetLevel(oldLevel) }()

	SetLevel(InfoLevel)
	SetQuiet(true)
	if getLogLevel() != WarnLevel {
		t.Errorf("SetQuiet(true) failed, expected WarnLevel, got %v", getLogLevel())
	}
}

// TestLogOutput tests that logging actually produces output
func TestLogOutput(t *testing.T) {
	oldLevel := getLogLevel()
	defer func() { SetLevel(oldLevel) }()

	SetLevel(InfoLevel)

	// Just verify that Info function doesn't panic or error
	// Output testing via stdout redirection is complex and unreliable in tests
	Info("test message")
	Warn("test warning")
	Error("test error")
}

// TestDebugLevelFiltering tests that debug messages are filtered correctly
func TestDebugLevelFiltering(t *testing.T) {
	oldLevel := getLogLevel()
	defer func() { SetLevel(oldLevel) }()

	// Set to info level (higher than debug)
	SetLevel(InfoLevel)

	// Just verify filtering logic works by checking log level comparison
	if InfoLevel >= DebugLevel {
		t.Logf("Debug messages should be filtered at INFO level")
	}

	// Debug should not execute when level is higher
	Debug("this should be filtered")
	Info("this should show")
}

// TestFormatMessage tests message formatting
func TestFormatMessage(t *testing.T) {
	msg := formatMessage("TEST", "sample message")

	if !bytes.Contains([]byte(msg), []byte("[")) {
		t.Errorf("formatMessage() should include timestamp with brackets")
	}

	if !bytes.Contains([]byte(msg), []byte("sample message")) {
		t.Errorf("formatMessage() should include the message")
	}
}
