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

func UnmountAll(execute bool, mountpoints []string) error {

	for _, mountpoint := range mountpoints {
		Execute(
			execute,
			"umount",
			mountpoint,
		)
	}

	return nil

}
