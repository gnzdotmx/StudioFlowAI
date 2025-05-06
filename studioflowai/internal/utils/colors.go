package utils

// Terminal color codes using ANSI escape sequences
const (
	ResetColor   = "\033[0m"
	RedColor     = "\033[31m" // For errors
	GreenColor   = "\033[32m" // For success/completion
	YellowColor  = "\033[33m" // For warnings
	BlueColor    = "\033[34m" // For module start
	MagentaColor = "\033[35m" // For emphasis
	CyanColor    = "\033[36m" // For info
)

// ColoredText wraps text with color codes and reset at the end
func ColoredText(text string, color string) string {
	return color + text + ResetColor
}

// Info returns blue-colored text for module info messages
func Info(text string) string {
	return ColoredText(text, BlueColor)
}

// Success returns green-colored text for success messages
func Success(text string) string {
	return ColoredText(text, GreenColor)
}

// Warning returns yellow-colored text for warning messages
func Warning(text string) string {
	return ColoredText(text, YellowColor)
}

// Error returns red-colored text for error messages
func Error(text string) string {
	return ColoredText(text, RedColor)
}

// Highlight returns magenta-colored text for emphasized content
func Highlight(text string) string {
	return ColoredText(text, MagentaColor)
}

// Debug returns cyan-colored text for debug info
func Debug(text string) string {
	return ColoredText(text, CyanColor)
}
