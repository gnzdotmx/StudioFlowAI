package cmd

import (
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"github.com/spf13/cobra"
)

var (
	// verbosityLevel is the command-line flag for setting the log level
	verbosityLevel string
)

var rootCmd = &cobra.Command{
	Use:   "studioflowai",
	Short: "An AI-powered video workflow tool for content creators",
	Long: `StudioFlowAI is a modular application for content creators
to process videos with AI-powered configurable workflows defined in YAML.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set the global log level based on the flag
		logLevel := utils.LogLevelFromString(verbosityLevel)
		utils.SetLogLevel(logLevel)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Initialize global flags
	rootCmd.PersistentFlags().StringVarP(&verbosityLevel, "log-level", "l", "normal",
		"Set the logging verbosity level: quiet, normal, verbose, debug")
}
