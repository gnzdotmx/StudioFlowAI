package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IsTextFile checks if a file is a text file and not binary
func IsTextFile(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		LogError("Error opening file %s: %v", filePath, err)
		return false
	}
	defer func() {
		if err := f.Close(); err != nil {
			LogWarning("Failed to close file: %v", err)
		}
	}()

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
	defer func() {
		if err := f.Close(); err != nil {
			LogWarning("Failed to close file: %v", err)
		}
	}()

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
	defer func() {
		if err := f.Close(); err != nil {
			LogWarning("Failed to close file: %v", err)
		}
	}()

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

// ExpandHomeDir expands a path if it starts with "~/"
func ExpandHomeDir(path string) (string, error) {
	if path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			LogWarning("Failed to close source file: %v", err)
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			LogWarning("Failed to close destination file: %v", err)
		}
	}()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// LoadEnvFile loads environment variables from .env file in the current directory
func LoadEnvFile() error {
	// Try to open .env file
	envFile, err := os.Open(".env")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no .env file found in current directory")
		}
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer func() {
		if err := envFile.Close(); err != nil {
			LogWarning("Failed to close env file: %v", err)
		}
	}()

	// Read file line by line
	scanner := bufio.NewScanner(envFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first = sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip malformed lines
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		// Set environment variable
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading .env file: %w", err)
	}

	return nil
}
