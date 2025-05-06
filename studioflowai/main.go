package main

import (
	"fmt"
	"os"

	"github.com/gnzdotmx/studioflowai/studioflowai/cmd"

	"github.com/joho/godotenv"
)

func init() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found - using environment variables")
	} else {
		fmt.Println("Loaded environment variables from .env file")
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
