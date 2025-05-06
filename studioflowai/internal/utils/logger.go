package utils

import (
	"fmt"
	"os"
	"strings"
)

// LogLevel represents the level of logging verbosity
type LogLevel int

const (
	// LevelQuiet suppresses all output except errors
	LevelQuiet LogLevel = iota
	// LevelNormal shows standard workflow progress
	LevelNormal
	// LevelVerbose shows detailed information about each step
	LevelVerbose
	// LevelDebug shows all debugging information
	LevelDebug
)

var (
	// CurrentLogLevel is the global log level setting
	CurrentLogLevel LogLevel = LevelNormal
)

// SetLogLevel sets the global logging level
func SetLogLevel(level LogLevel) {
	CurrentLogLevel = level
}

// LogLevelFromString converts a string level name to LogLevel
func LogLevelFromString(level string) LogLevel {
	switch strings.ToLower(level) {
	case "quiet", "q":
		return LevelQuiet
	case "normal", "n":
		return LevelNormal
	case "verbose", "v":
		return LevelVerbose
	case "debug", "d":
		return LevelDebug
	default:
		return LevelNormal
	}
}

// LogError logs an error message (always shown)
func LogError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s\n", Error(fmt.Sprintf(format, args...)))
}

// LogInfo logs an informational message at Normal+ level
func LogInfo(format string, args ...interface{}) {
	if CurrentLogLevel >= LevelNormal {
		fmt.Printf("%s\n", Info(fmt.Sprintf(format, args...)))
	}
}

// LogSuccess logs a success message at Normal+ level
func LogSuccess(format string, args ...interface{}) {
	if CurrentLogLevel >= LevelNormal {
		fmt.Printf("%s\n", Success(fmt.Sprintf(format, args...)))
	}
}

// LogVerbose logs a message at Verbose+ level
func LogVerbose(format string, args ...interface{}) {
	if CurrentLogLevel >= LevelVerbose {
		fmt.Printf("\t%s\n", Info(fmt.Sprintf(format, args...)))
	}
}

// LogDebug logs a debug message at Debug level
func LogDebug(format string, args ...interface{}) {
	if CurrentLogLevel >= LevelDebug {
		fmt.Printf("\t%s\n", Debug(fmt.Sprintf(format, args...)))
	}
}

// LogWarning logs a warning message at Normal+ level
func LogWarning(format string, args ...interface{}) {
	if CurrentLogLevel >= LevelNormal {
		fmt.Printf("%s\n", Warning(fmt.Sprintf(format, args...)))
	}
}
