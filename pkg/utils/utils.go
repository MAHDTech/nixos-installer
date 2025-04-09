// Package utils provides utilities for the installer.
package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// IsValidBlockDevice function will return true if the device is a valid block device.
func IsValidBlockDevice(device string) bool {
	// Check if the device exists
	_, err := os.Stat(device)
	if os.IsNotExist(err) {
		return false
	}

	// Check if it's a block device
	info, err := os.Stat(device)
	if err != nil {
		return false
	}

	// Use the mode bits to determine if it's a block device
	return (info.Mode() & os.ModeDevice) == os.ModeDevice

}

// Execute function will execute a command and check for errors.
// It returns an error if the command cannot be found or fails to run.
func Execute(execute bool, cmdName string, args ...string) error {
	// Verify the command exists in PATH
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return fmt.Errorf("command not found %s: %w", cmdName, err)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if execute {
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to execute command %s: %w", cmd.String(), err)
		}
	}
	log.Printf("DRY RUN: Would run %s\n", cmd.String())
	return nil // Command executed successfully or dry run is considered successful
}

// ExecuteSilent function will execute a command and ignore any errors during run.
// It still returns an error if the command cannot be found.
func ExecuteSilent(execute bool, cmdName string, args ...string) error {
	// Verify the command exists in PATH
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return fmt.Errorf("command not found %s: %w", cmdName, err)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if execute {
		err := cmd.Run()
		if err != nil {
			// Log the error but don't return it, as per function's purpose
			log.Printf("Command failed (but ignored): %s, Error: %s", cmd.String(), err)
		}
	}
	log.Printf("DRY RUN: Would run %s\n", cmd.String())
	return nil // Even if the command failed, the function's contract is met or dry run is considered successful
}

// ExecuteStdOut function will execute a command and return the stdout.
// It returns an error if the command cannot be found or fails to run.
func ExecuteStdOut(execute bool, cmdName string, args ...string) (string, error) {
	// Verify the command exists in PATH
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return "", fmt.Errorf("command not found %s: %w", cmdName, err)
	}

	cmd := exec.Command(path, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if execute {
		output, err := cmd.Output() // Captures Stdout
		if err != nil {
			// If cmd.Output fails, err is *exec.ExitError which contains Stderr
			return "", fmt.Errorf(
				"failed to execute command %s and capture output: %w",
				cmd.String(),
				err,
			)
		}
		return string(output), nil // Command executed successfully
	}
	log.Printf("DRY RUN: Would run %s\n", cmd.String())
	return "", nil // Dry run is considered successful, returns empty string and no error
}

// FileExists function will return true if the file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
