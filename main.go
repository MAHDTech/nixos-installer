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

// Run the NixOS Installer.
func main() {
	// Parse command line flags
	var (
		logFile      = flag.String("log-file", "nixos-installer.log", "Path to log file")
		consoleLevel = flag.String("console-level", "INFO", "Console log level (DEBUG, INFO, WARN, ERROR)")
		fileLevel    = flag.String("file-level", "DEBUG", "File log level (DEBUG, INFO, WARN, ERROR)")
		showHelp     = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

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
		return
	}

	// Parse log levels
	consoleLogLevel, err := parseLogLevel(*consoleLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid console log level: %v\n", err)
		os.Exit(1)
	}

	fileLogLevel, err := parseLogLevel(*fileLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid file log level: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := sysutil.InitLogger(*logFile, consoleLogLevel, fileLogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := sysutil.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	sysutil.Info("Starting NixOS Installer...")

	err = installer.Run()
	if err != nil {
		sysutil.Error("Installation failed with error: %v", err)
		os.Exit(1)
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
