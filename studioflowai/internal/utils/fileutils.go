package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// IsTextFile checks if a file is a text file and not binary
func IsTextFile(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		LogError("Error opening file %s: %v", filePath, err)
		return false
	}
	defer f.Close()

	// Read the first 512 bytes to determine content type
	buffer := make([]byte, 512)
	n, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}

	// Check for binary indicators
	for i := 0; i < n; i++ {
		// Check for null bytes and other control characters (except common ones like tab, newline)
		if (buffer[i] < 9 || (buffer[i] > 13 && buffer[i] < 32)) && buffer[i] != 0x1B {
			LogWarning("File %s appears to be binary (detected binary content)", filePath)
			return false
		}
	}

	return true
}

// ReadTextFile reads a file line by line to ensure it's text
func ReadTextFile(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	LogDebug("Read %d lines from %s", len(lines), filePath)
	return strings.Join(lines, "\n"), nil
}

// WriteTextFile writes text to a file, ensuring it's written as text
func WriteTextFile(filePath string, content string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	if _, err := writer.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	LogDebug("Successfully wrote content to %s", filePath)
	return nil
}
