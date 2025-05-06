package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	outputDir     string
	keepLatest    int
	olderThanDays int
	cleanupDryRun bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up old workflow output directories",
	Long:  `Remove old workflow run folders based on age or count.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if outputDir == "" {
			return fmt.Errorf("output directory is required")
		}

		// Check if output directory exists
		if _, err := os.Stat(outputDir); os.IsNotExist(err) {
			return fmt.Errorf("output directory %s does not exist", outputDir)
		}

		// Get all subdirectories
		entries, err := os.ReadDir(outputDir)
		if err != nil {
			return fmt.Errorf("failed to read output directory: %w", err)
		}

		// Filter for workflow run directories (they should have a specific format)
		var workflowDirs []string
		for _, entry := range entries {
			if entry.IsDir() && strings.Contains(entry.Name(), "-") {
				workflowDirs = append(workflowDirs, entry.Name())
			}
		}

		if len(workflowDirs) == 0 {
			fmt.Println("No workflow run directories found.")
			return nil
		}

		// Sort directories by timestamp (newest last)
		sort.Slice(workflowDirs, func(i, j int) bool {
			// Extract timestamps from directory names
			iParts := strings.Split(workflowDirs[i], "-")
			jParts := strings.Split(workflowDirs[j], "-")

			// If we can't parse timestamps, fall back to string comparison
			if len(iParts) < 2 || len(jParts) < 2 {
				return workflowDirs[i] < workflowDirs[j]
			}

			return iParts[len(iParts)-2] < jParts[len(jParts)-2] ||
				(iParts[len(iParts)-2] == jParts[len(jParts)-2] &&
					iParts[len(iParts)-1] < jParts[len(jParts)-1])
		})

		// Determine which directories to delete
		var toDelete []string

		// If keep-latest is specified
		if keepLatest > 0 && len(workflowDirs) > keepLatest {
			toDelete = append(toDelete, workflowDirs[:len(workflowDirs)-keepLatest]...)
		}

		// If older-than is specified
		if olderThanDays > 0 {
			cutoffTime := time.Now().AddDate(0, 0, -olderThanDays)

			for _, dir := range workflowDirs {
				// Try to extract the timestamp
				parts := strings.Split(dir, "-")
				if len(parts) >= 2 {
					// Try to parse the timestamp (format: YYYYMMDD-HHMMSS)
					dateStr := parts[len(parts)-2]
					timeStr := parts[len(parts)-1]

					if len(dateStr) == 8 && len(timeStr) == 6 {
						year, _ := strconv.Atoi(dateStr[:4])
						month, _ := strconv.Atoi(dateStr[4:6])
						day, _ := strconv.Atoi(dateStr[6:8])
						hour, _ := strconv.Atoi(timeStr[:2])
						minute, _ := strconv.Atoi(timeStr[2:4])
						second, _ := strconv.Atoi(timeStr[4:6])

						dirTime := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)

						if dirTime.Before(cutoffTime) && !contains(toDelete, dir) {
							toDelete = append(toDelete, dir)
						}
					}
				}
			}
		}

		// Delete directories
		if len(toDelete) == 0 {
			fmt.Println("No directories to delete.")
			return nil
		}

		fmt.Printf("Found %d directories to delete:\n", len(toDelete))
		for _, dir := range toDelete {
			fmt.Printf("- %s\n", dir)
		}

		if cleanupDryRun {
			fmt.Println("Dry run - no directories were deleted.")
			return nil
		}

		// Actually delete the directories
		for _, dir := range toDelete {
			fullPath := filepath.Join(outputDir, dir)
			fmt.Printf("Deleting %s...\n", fullPath)

			if err := os.RemoveAll(fullPath); err != nil {
				fmt.Printf("Error deleting %s: %v\n", fullPath, err)
			}
		}

		fmt.Println("Cleanup completed.")
		return nil
	},
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func init() {
	cleanupCmd.Flags().StringVarP(&outputDir, "dir", "d", "", "Output directory to clean up (required)")
	cleanupCmd.Flags().IntVarP(&keepLatest, "keep-latest", "k", 0, "Keep this many latest directories")
	cleanupCmd.Flags().IntVarP(&olderThanDays, "older-than", "o", 0, "Delete directories older than this many days")
	cleanupCmd.Flags().BoolVarP(&cleanupDryRun, "dry-run", "n", false, "Show what would be deleted without actually deleting")

	_ = cleanupCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(cleanupCmd)
}
