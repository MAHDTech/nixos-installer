// Main entry point for the NixOS Installer.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	installer "github.com/MAHDTech/nixos-installer/pkg/installer"
	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
)

// Build-time variables injected by ldflags
var (
	Version   = "dev"
	CommitSHA = "unknown"
	BuildDate = "unknown"
)

// Run the NixOS Installer.
func main() {
	// Parse command line flags
	var (
		// Installer flags
		configFile = flag.String("config", "config.yaml", "Path to the YAML configuration file")
		execute    = flag.Bool(
			"run",
			false,
			"Execute mode (defaults to false which will run in dry-run mode)",
		)
		executeInstall = flag.Bool(
			"install",
			false,
			"Enable to automatically install NixOS (defaults to false which only generates the NixOS configuration)",
		)

		// Logging flags
		logFile      = flag.String("log-file", "nixos-installer.log", "Path to log file")
		consoleLevel = flag.String(
			"console-level",
			"WARN",
			"Console log level (DEBUG, INFO, WARN, ERROR)",
		)
		fileLevel = flag.String(
			"file-level",
			"DEBUG",
			"File log level (DEBUG, INFO, WARN, ERROR)",
		)

		// Help and version flags
		showHelp    = flag.Bool("help", false, "Show help message")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("NixOS Installer v%s\n", Version)
		fmt.Printf("Commit: %s\n", CommitSHA)
		fmt.Printf("Built: %s\n", BuildDate)
		return
	}

	if *showHelp {
		fmt.Println("NixOS Installer - Automated NixOS installation with ZFS")
		fmt.Println()
		fmt.Println("Usage:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Log Levels:")
		fmt.Println("  DEBUG - Detailed debug information")
		fmt.Println("  INFO  - General information messages")
		fmt.Println("  WARN  - Warning messages")
		fmt.Println("  ERROR - Error messages only")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./nixos-installer -config=my-config.yaml")
		fmt.Println("  ./nixos-installer -config=my-config.yaml -run -install")
		fmt.Println("  ./nixos-installer -console-level=WARN -file-level=DEBUG")
		fmt.Println()
		fmt.Println("Configuration Options:")
		fmt.Println("  -config can be a file path or a config name to fetch from GitHub")
		fmt.Println("  Examples: -config=./config.yaml or -config=HYPERVISOR-1")
		return
	}

	// Initialize logger early to ensure defer works properly
	var exitCode int
	defer func() {
		if err := sysutil.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	// Parse log levels
	consoleLogLevel, err := parseLogLevel(*consoleLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid console log level: %v\n", err)
		exitCode = 1
		return
	}

	fileLogLevel, err := parseLogLevel(*fileLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid file log level: %v\n", err)
		exitCode = 1
		return
	}

	// Initialize logger
	if err := sysutil.InitLogger(*logFile, consoleLogLevel, fileLogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		exitCode = 1
		return
	}

	sysutil.Info(
		"Starting NixOS Installer version: %s, commit: %s, built: %s",
		Version,
		CommitSHA,
		BuildDate,
	)

	// Log execution mode
	if *execute {
		sysutil.Info("Running in execute mode")
	} else {
		sysutil.Info("Running in dry run mode, see '-help' for more information")
	}

	err = installer.Run(*configFile, *execute, *executeInstall)
	if err != nil {
		sysutil.Error("Installation failed with error: %v", err)
		exitCode = 1
		return
	}

	sysutil.Success("Installation completed successfully!")
}

// parseLogLevel converts a string to LogLevel
func parseLogLevel(level string) (sysutil.LogLevel, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return sysutil.DEBUG, nil
	case "INFO":
		return sysutil.INFO, nil
	case "WARN":
		return sysutil.WARN, nil
	case "ERROR":
		return sysutil.ERROR, nil
	default:
		return sysutil.INFO, fmt.Errorf("unknown log level: %s", level)
	}
}
