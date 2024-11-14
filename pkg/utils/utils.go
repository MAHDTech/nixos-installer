// Package utils provides utilities for the installer.
package utils

import (
	"log"
	"os"
	"os/exec"

	validate "github.com/MAHDTech/nixos-installer/pkg/validate"
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
func Execute(execute bool, cmdName string, args ...string) {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if execute {
		validate.Panic(cmd.Run())
	} else {
		log.Printf("DRY RUN: Would run %s\n", cmd.String())
	}
}

// ExecuteSilent function will execute a command and ignore any errors.
func ExecuteSilent(execute bool, cmdName string, args ...string) {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if execute {
		err := cmd.Run()
		if err != nil {
			log.Printf("Command failed, but continuing: %s", err)
		}
	} else {
		log.Printf("DRY RUN: Would run %s\n", cmd.String())
	}
}

// ExecuteStdOut function will execute a command and return the stdout.
func ExecuteStdOut(execute bool, cmdName string, args ...string) string {
	cmd := exec.Command(cmdName, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if !execute {
		output, err := cmd.Output()
		validate.Error(err)
		return string(output)
	}

	log.Printf("DRY RUN: Would run %s\n", cmd.String())
	return ""

}

// FileExists function will return true if the file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
