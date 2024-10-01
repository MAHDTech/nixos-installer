package utils

import (
	"log"
	"os"
	"os/exec"

	validate "github.com/MAHDTech/nixos-installer/pkg/validate"
)

// Determines if a device is a valid block device.
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

// Executes a command and check for errors.
func Execute(dryRun *bool, cmdName string, args ...string) {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if *dryRun {
		log.Printf("DRY RUN: Would run %s\n", cmd.String())
	} else {
		validate.Panic(cmd.Run())
	}
}
