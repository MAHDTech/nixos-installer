// Package sysutil provides utilities for working with filesystem mount operations.
package sysutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Device represents a block device with mountpoints
type Device struct {
	ID          string   `json:"id"`
	Mountpoints []string `json:"mountpoints"`
}

// BlockDevices represents the JSON structure returned by lsblk
type BlockDevices struct {
	Blockdevices []Device `json:"blockdevices"`
}

// GetMountpoints function will return all mountpoints for a given device ID.
func GetMountpoints(deviceID string, data []byte) ([]string, error) {
	var blockDevices BlockDevices
	mountpoints := []string{}

	err := json.Unmarshal(data, &blockDevices)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block device JSON: %w", err)
	}

	// Normalize for comparison.
	deviceID = strings.TrimPrefix(deviceID, "/dev/disk/by-id/")
	deviceID = strings.TrimPrefix(deviceID, "usb-")
	deviceID = strings.TrimPrefix(deviceID, "nvme-")
	deviceID = strings.TrimSpace(deviceID)
	deviceID = strings.ToLower(deviceID)

	Debug("Looking for device ID: %s (normalized: %s)", deviceID, deviceID)

	// Unmarshal the JSON into blockDevices.
	for _, device := range blockDevices.Blockdevices {
		// Skip if device.ID is null
		if device.ID == "" {
			continue
		}

		// Normalize for comparison.
		deviceIDFromJSON := strings.TrimSpace(device.ID)
		deviceIDFromJSON = strings.ToLower(deviceIDFromJSON)

		// Look for the device ID inside the JSON.
		if strings.Contains(deviceIDFromJSON, deviceID) {
			Debug("Checking device ID %s for mountpoints", deviceIDFromJSON)
			if device.Mountpoints != nil {
				for _, mountpoint := range device.Mountpoints {
					if mountpoint != "" {
						Debug("Found a mountpoint for %s at %s", deviceIDFromJSON, mountpoint)
						mountpoints = append(mountpoints, mountpoint)
					}
				}
			}
		}
	}

	return mountpoints, nil
}

// UnmountAll function will unmount all given mountpoints.
func UnmountAll(execute bool, mountpoints []string) error {
	// Loop over each mountpoint and unmount it.
	for _, mountpoint := range mountpoints {
		Info("Unmounting %s", mountpoint)
		_, err := Execute(
			execute,
			ModeNormal,
			"umount",
			"-R", // Recursive unmount
			mountpoint,
		)
		// Return immediately if any unmount fails
		if err != nil {
			return fmt.Errorf("failed to unmount %s: %w", mountpoint, err)
		}
	}
	// This assumes all unmounts succeeded or were in dry-run mode.
	return nil
}
