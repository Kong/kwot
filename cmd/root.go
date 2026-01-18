package cmd

import (
	"fmt"
	"os"

	"github.com/Kong/kwot/internal/config"
	"github.com/Kong/kwot/internal/logger"
	"github.com/spf13/cobra"
)

func init() {
	// Suppress audit logging to prevent permission errors
	// Kong tries to write to /var/log/ which requires root privileges
	_ = os.Setenv("KONG_LOG_LEVEL", "warn")
}

var (
	cfgFile       string
	workspaceName string
	dryRun        bool
	verbose       bool
	quiet         bool
	force         bool
	versionFlag   bool
	versionString string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "kwot",
	Short: "Kong Workspace Onboarding & Configuration Tie",
	Long: `A CLI tool to manage Kong Gateway workspaces, groups, roles, and RBAC configurations.

This tool automates the configuration of Kong Gateway Enterprise environments,
including workspaces, RBAC roles, and group assignments. Ties together workspaces,
access controls, and permissions into a single declarative configuration.

Common commands:
  kwot diff                      Show drift between config and Kong
  kwot apply                     Apply all configuration (workspaces, roles, groups)
  kwot apply --dry-run           Preview changes before applying
  kwot apply --workspace <name>  Apply to a specific workspace
  kwot delete --force            Delete ALL resources (requires flags)
  kwot delete workspaces -n <ws> Delete a specific workspace`,
	DisableSuggestions:         false,
	SuggestionsMinimumDistance: 1,
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			printBanner()
			fmt.Printf("Version: %s\n", versionString)
			return
		}
		_ = cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Disable the completion command as it's not fully implemented
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .env)")
	rootCmd.PersistentFlags().StringVarP(&workspaceName, "workspace", "w", "all", "workspace name (default is 'all')")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "preview changes without applying them")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show detailed output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress all output except errors")
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "", false, "show version information")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Configure logger based on flags
	if verbose {
		logger.SetVerbose(true)
	}
	if quiet {
		logger.SetQuiet(true)
	}

	// Skip config loading for version and help flags
	// These don't require Kong authentication
	if versionFlag {
		return
	}
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" || arg == "help" {
			return
		}
	}

	if err := config.LoadConfig(cfgFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
}

// SetVersion sets the version string for display
func SetVersion(v string) {
	versionString = v
}

// printBanner displays the kwot ASCII art banner
func printBanner() {
	fmt.Println()
	fmt.Println("██╗  ██╗██╗    ██╗ ██████╗ ████████╗")
	fmt.Println("██║ ██╔╝██║    ██║██╔═══██╗╚══██╔══╝")
	fmt.Println("█████╔╝ ██║ █╗ ██║██║   ██║   ██║   ")
	fmt.Println("██╔═██╗ ██║███╗██║██║   ██║   ██║   ")
	fmt.Println("██║  ██╗╚███╔███╔╝╚██████╔╝   ██║   ")
	fmt.Println("╚═╝  ╚═╝ ╚══╝╚══╝  ╚═════╝    ╚═╝   ")
	fmt.Println()
	fmt.Print("\033[4mK\033[24mong \033[4mW\033[24morkspace \033[4mO\033[24mnboarding \033[4mT\033[24mool\n")
}
