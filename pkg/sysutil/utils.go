// Package sysutil provides core utilities for command execution, filesystem operations,
// and device validation used throughout the nixos-installer.
package sysutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ExecuteMode defines how the Execute function handles command execution
type ExecuteMode int

const (
	// ModeNormal runs the command and returns any errors
	ModeNormal ExecuteMode = iota
	// ModeSilent runs the command and only logs errors without returning them
	ModeSilent
	// ModeStdOut captures and returns stdout from the command
	ModeStdOut
	// ModeStdErr captures and returns stderr from the command
	ModeStdErr
)

// Execute function will execute a command with the specified mode.
// It returns stdout (if requested) and an error if the command cannot be found or fails to run.
func Execute(
	execute bool,
	mode ExecuteMode,
	cmdName string,
	args ...string,
) (string, error) {

	// Verify the command exists in PATH
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return "", fmt.Errorf("command not found %s: %w", cmdName, err)
	}

	cmd := exec.Command(path, args...) // #nosec G204

	// Configure command IO based on mode
	if mode == ModeStdOut {
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
	} else if mode == ModeStdErr {
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
	}

	Debug("%s: %s",
		map[bool]string{true: "EXECUTING", false: "DRY RUN"}[execute],
		cmd.String(),
	)

	// Skip execution if we're in dry run mode
	if !execute {
		return "", nil
	}

	// Execute based on the selected mode
	switch mode {

	case ModeNormal:
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to execute command %s: %w", cmd.String(), err)
		}
		return "", nil

	case ModeSilent:
		if err := cmd.Run(); err != nil {
			Warn("Command failed (but ignored): %s, Error: %s", cmd.String(), err)
		}
		return "", nil

	case ModeStdOut:
		output, err := cmd.Output() // Capture the Stdout
		if err != nil {
			return "", fmt.Errorf(
				"failed to execute command %s and capture output: %w",
				cmd.String(),
				err,
			)
		}
		return string(output), nil

	case ModeStdErr:
		// Capture stderr by redirecting it to a pipe
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return "", fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("failed to start command %s: %w", cmd.String(), err)
		}

		// Read stderr
		stderrBytes, err := io.ReadAll(stderr)
		if err != nil {
			return "", fmt.Errorf("failed to read stderr: %w", err)
		}

		// Wait for command to complete
		if err := cmd.Wait(); err != nil {
			return string(stderrBytes), fmt.Errorf("failed to execute command %s: %w", cmd.String(), err)
		}

		return string(stderrBytes), nil
	}

	// This should never happen if the mode is valid
	return "", fmt.Errorf("invalid execution mode: %d", mode)
}

// IsValidBlockDevice function will return true if the device is a valid block device.
func IsValidBlockDevice(device string) bool {
	// Check if the device exists
	deviceInfo, err := os.Stat(device)
	if err != nil {
		return false
	}

	// Make sure the ModeDevice bit is set, but not the ModeDir or ModeRegular bits
	return (deviceInfo.Mode() & os.ModeDevice) == os.ModeDevice
}

// FileExists function will return true if the file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
