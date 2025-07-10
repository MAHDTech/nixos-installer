// Package utils provides utilities for working with filesystem mount operations.
package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// BlockDevice represents the structure of a block device in the JSON.
type BlockDevice struct {
	ID          string   `json:"id"`
	Mountpoints []string `json:"mountpoints"`
}

// BlockDevices is a struct to hold the top-level "blockdevices" array.
type BlockDevices struct {
	Blockdevices []BlockDevice `json:"blockdevices"`
}

// GetMountpoints function will return all mountpoints for a given device ID.
func GetMountpoints(deviceID string, data []byte) ([]string, error) {

	var blockdevices BlockDevices

	// Normalise for comparison.
	deviceID = strings.TrimPrefix(deviceID, "/dev/disk/by-id/")
	deviceID = strings.TrimPrefix(deviceID, "usb-")
	deviceID = strings.TrimPrefix(deviceID, "nvme-")
	deviceID = strings.TrimSpace(deviceID)
	deviceID = strings.ToLower(deviceID)

	// Unmarshal the JSON into blockdevices.
	err := json.Unmarshal(data, &blockdevices)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	mountpoints := []string{}
	for _, device := range blockdevices.Blockdevices {

		// Skip if device.ID is null
		if device.ID == "" {
			continue
		}

		// Normalise for comparison.
		deviceIDFromJSON := strings.TrimSpace(device.ID)
		deviceIDFromJSON = strings.ToLower(deviceIDFromJSON)

		// Look for the device ID inside the JSON.
		if strings.Contains(deviceIDFromJSON, deviceID) {
			log.Printf("Checking device ID %s for mountpoints", deviceIDFromJSON)
			if device.Mountpoints != nil {
				for _, mountpoint := range device.Mountpoints {
					if mountpoint != "" {
						log.Printf("Found a mountpoint for %s at %s", deviceIDFromJSON, mountpoint)
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
		log.Printf("Unmounting %s\n", mountpoint)
		_, err := Execute(
			execute,
			ModeNormal,
			"umount",
			"-R", // Recursive unmount
			mountpoint,
		)
		if err != nil {
			// Return immediately if any unmount fails
			return fmt.Errorf("failed to unmount %s: %w", mountpoint, err)
		}
	}
	// This assumes all unmounts succeeded or were in dry-run mode.
	return nil
}
