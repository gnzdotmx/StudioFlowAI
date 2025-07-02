package utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ShortClip represents a single short video clip
type ShortClip struct {
	Title       string `yaml:"title"`
	StartTime   string `yaml:"startTime"`
	EndTime     string `yaml:"endTime"`
	Description string `yaml:"description"`
	Tags        string `yaml:"tags"`
	ShortTitle  string `yaml:"shortTitle"`
}

// ShortsData represents the structure of the shorts_suggestions.yaml file
type ShortsData struct {
	SourceVideo string      `yaml:"sourceVideo"`
	Shorts      []ShortClip `yaml:"shorts"`
}

// readShortsFile reads and parses the shorts_suggestions.yaml file
func ReadShortsFile(filePath string) (*ShortsData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var shortsData ShortsData
	if err := yaml.Unmarshal(data, &shortsData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &shortsData, nil
}

// listShorts lists available shorts that can be uploaded
func ListShorts(shortsData *ShortsData) error {
	LogInfo("Available shorts for upload:")
	for i, short := range shortsData.Shorts {
		LogInfo("%d. Title: %s", i+1, short.ShortTitle)
		LogInfo("   Duration: %s - %s", short.StartTime, short.EndTime)
		LogInfo("   Description: %s", short.Description)
		LogInfo("   Tags: %s", short.Tags)
		LogInfo("---")
	}
	return nil
}
