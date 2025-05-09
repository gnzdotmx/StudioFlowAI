package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gnzdotmx/studioflowai/studioflowai/cmd"

	"github.com/joho/godotenv"
)

func init() {
	// First try to load from global config in user's home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfigPath := filepath.Join(homeDir, ".studioflowai", ".env")
		if err := godotenv.Load(globalConfigPath); err == nil {
			fmt.Println("Loaded environment variables from global config file")
		}
	}

	// Then try to load from local .env file if it exists
	if err := godotenv.Load(); err != nil {
		fmt.Println("No local .env file found - using environment variables")
	} else {
		fmt.Println("Loaded environment variables from local .env file")
	}

	// Debug: Check if the API key is set
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		fmt.Println("OPENAI_API_KEY is set and has length:", len(apiKey))
	} else {
		fmt.Println("OPENAI_API_KEY is not set")
	}
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
