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

	// Normalize the device ID for comparison
	// Remove /dev/disk/by-id/ prefix if present
	normalizedDeviceID := deviceID
	if strings.HasPrefix(deviceID, "/dev/disk/by-id/") {
		normalizedDeviceID = strings.TrimPrefix(deviceID, "/dev/disk/by-id/")
	}

	// Also remove common prefixes that lsblk might not include
	normalizedDeviceID = strings.TrimPrefix(normalizedDeviceID, "usb-")
	normalizedDeviceID = strings.TrimPrefix(normalizedDeviceID, "nvme-")

	Debug("Looking for device ID: %s (normalized: %s)", deviceID, normalizedDeviceID)

	// Find the device with the matching ID
	var deviceIDFromJSON string
	for _, device := range blockDevices.Blockdevices {
		// Skip if device.ID is null
		if device.ID == "" {
			continue
		}

		// Normalize the device ID from JSON for comparison
		normalizedJSONID := strings.TrimSpace(device.ID)

		// Remove common prefixes from JSON ID for comparison
		normalizedJSONID = strings.TrimPrefix(normalizedJSONID, "usb-")
		normalizedJSONID = strings.TrimPrefix(normalizedJSONID, "nvme-")

		Debug("Comparing with JSON device ID: %s (normalized: %s)", device.ID, normalizedJSONID)

		// Check for exact match or if the normalized device ID contains the JSON ID
		if normalizedJSONID == normalizedDeviceID || strings.Contains(normalizedJSONID, normalizedDeviceID) {
			deviceIDFromJSON = device.ID
			Debug("Found matching device ID %s for mountpoints", deviceIDFromJSON)
			if device.Mountpoints != nil {
				for _, mountpoint := range device.Mountpoints {
					if mountpoint != "" {
						Debug("Found a mountpoint for %s at %s", deviceIDFromJSON, mountpoint)
						mountpoints = append(mountpoints, mountpoint)
					}
				}
			}
			break
		}
	}

	if deviceIDFromJSON == "" {
		return nil, fmt.Errorf("device ID %s not found in block device list", deviceID)
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
