package installer

import (
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// PartitionInfo holds information about created partitions
type PartitionInfo struct {
	UEFI        string   // UEFI/ESP partition
	NixOSConfig string   // NixOS configuration partition
	ZFSBoot     []string // ZFS boot partitions can be one or more
	ZFSData     []string // ZFS data/root partitions can be one or more
}

// unmountDisks unmounts all partitions on the specified disks in the config.
func unmountDisks(execute bool, configData *config.Config) error {
	var err error

	log.Println("--- Unmounting Disks ---")

	// First try to unmount everything using Linux kernel's force unmount
	// This is more reliable than ZFS's unmount for busy filesystems
	err = forceUnmountMountpoints(execute)
	if err != nil {
		// Non-fatal error, just log it
		log.Printf("Warning: Failed to force unmount mountpoints: %v", err)
	}

	log.Println("Processing ZFS Pools")

	// First get a list of all pools
	poolsOutput, err := utils.Execute(
		execute,
		utils.ModeStdOut,
		"zpool",
		"list",
		"-H",
		"-o",
		"name",
	)
	if err == nil && poolsOutput != "" {
		pools := strings.Split(strings.TrimSpace(poolsOutput), "\n")

		// For each pool, properly unmount all datasets recursively
		for _, pool := range pools {
			if pool == "" {
				continue
			}

			log.Printf("Unmounting all datasets in pool: %s", pool)

			// First list all datasets in reverse order (children first)
			datasetsOutput, listErr := utils.Execute(
				execute,
				utils.ModeStdOut,
				"zfs",
				"list",
				"-H",
				"-o",
				"name",
				"-r",
				"-t",
				"filesystem,volume",
				pool,
			)

			if listErr == nil && datasetsOutput != "" {
				// Split into individual datasets and reverse the order
				datasets := strings.Split(strings.TrimSpace(datasetsOutput), "\n")
				for i, j := 0, len(datasets)-1; i < j; i, j = i+1, j-1 {
					datasets[i], datasets[j] = datasets[j], datasets[i]
				}

				// Unmount each dataset individually
				for _, dataset := range datasets {
					if dataset == "" {
						continue
					}
					log.Printf("Unmounting dataset: %s", dataset)
					_, unmountErr := utils.Execute(
						execute,
						utils.ModeNormal,
						"zfs",
						"unmount",
						dataset,
					)
					if unmountErr != nil {
						log.Printf("Warning: Failed to unmount dataset %s: %v", dataset, unmountErr)
						// Try force unmount if regular unmount fails
						_, forceErr := utils.Execute(
							execute,
							utils.ModeNormal,
							"zfs",
							"unmount",
							"-f",
							dataset,
						)
						if forceErr != nil {
							log.Printf(
								"Warning: Force unmount also failed for %s: %v",
								dataset,
								forceErr,
							)
						}
					}
				}
			}

			// Now export the pool
			log.Printf("Exporting pool: %s", pool)
			_, exportErr := utils.Execute(execute, utils.ModeNormal, "zpool", "export", "-f", pool)
			if exportErr != nil {
				log.Printf("Warning: Failed to export pool %s: %v", pool, exportErr)
			}
		}
	} else {
		// Fallback to the original approach
		_, err = utils.Execute(execute, utils.ModeNormal, "zfs", "unmount", "-a", "-f")
		if err != nil {
			log.Printf("Warning: Failed to unmount ZFS pools: %v", err)
		}
		time.Sleep(3 * time.Second) // Give time for unmounts to finish

		// Process ZFS export
		_, err = utils.Execute(execute, utils.ModeNormal, "zpool", "export", "-a", "-f")
		if err != nil {
			log.Printf("Warning: Failed to export ZFS pools: %v", err)
		}
		time.Sleep(3 * time.Second) // Give time for exports to finish
	}

	log.Println("Processing other mounts")

	// Create a slice to store all mountpoints
	var allMountPoints []string

	// Get all block devices as JSON
	// Need to always run this even in dry-run mode
	blockDevicesJSON, err := utils.Execute(
		true,
		utils.ModeStdOut,
		"lsblk",
		"--noheadings",
		"--json",
		"--output",
		"ID,MOUNTPOINTS",
	)
	if err != nil {
		return fmt.Errorf("failed to get block device information: %w", err)
	}

	// Check if the UEFI disk is in the config
	if configData.UEFI.Disk != "" {

		log.Printf("Checking for mountpoints on UEFI disk: %s", configData.UEFI.Disk)

		uefiMountPoints, err := utils.GetMountpoints(configData.UEFI.Disk, []byte(blockDevicesJSON))
		if err != nil {
			return fmt.Errorf(
				"failed to get mountpoints for UEFI disk %s: %w",
				configData.UEFI.Disk,
				err,
			)
		}

		// Add UEFI mountpoints to our list
		for _, mp := range uefiMountPoints {
			log.Printf("Found mountpoint on UEFI disk: %s", mp)
			allMountPoints = append(allMountPoints, mp)
		}
	}

	// Get all mountpoints for the ZFS disks and append them to the mountPoints slice
	for _, zfsDisk := range configData.ZFS.Disks {

		log.Printf("Checking for mountpoints on ZFS disk: %s", zfsDisk)

		zfsMountPoints, err := utils.GetMountpoints(zfsDisk, []byte(blockDevicesJSON))
		if err != nil {
			return fmt.Errorf("failed to get mountpoints for ZFS disk %s: %w", zfsDisk, err)
		}

		// Add ZFS disk mountpoints to our list
		for _, mp := range zfsMountPoints {
			log.Printf("Found mountpoint on ZFS disk: %s", mp)
			allMountPoints = append(allMountPoints, mp)
		}
	}

	// If mountpoints have been found, unmount them
	if len(allMountPoints) > 0 {
		log.Printf("Found %d mountpoints to unmount", len(allMountPoints))
		err = utils.UnmountAll(execute, allMountPoints)
		if err != nil {
			return fmt.Errorf("failed to unmount all mountpoints: %w", err)
		}
	} else {
		log.Println("No mountpoints found, nothing to unmount")
	}

	log.Println("--- Unmounting Disks Complete ---")
	return nil
}

// partitionDisks handles partitioning the specified disks according to the configuration.
// Returns partition info and an error if any partitioning step fails.
func partitionDisks(
	execute bool,
	configData *config.Config,
) (PartitionInfo, error) {
	partInfo := PartitionInfo{}
	var err error

	log.Println("--- Partitioning Disks ---")

	/*
	 --- UEFI Disk ---
	*/
	uefiDiskConfig := configData.UEFI
	partInfo.UEFI, err = partitionUEFIDisk(execute, configData)
	if err != nil {
		return PartitionInfo{}, fmt.Errorf(
			"failed to partition UEFI disk %s: %w",
			uefiDiskConfig.Disk,
			err,
		)
	}

	/*
	 --- NixOS Config Disk ---

	 (if enabled, uses same disk as UEFI)
	*/
	nixosConfigDisk := configData.UEFI.Disk
	if configData.NixOS.Config.Enabled {
		partInfo.NixOSConfig, err = partitionNixOSConfigDisk(
			execute,
			nixosConfigDisk,
			uefiDiskConfig.Disk,
		)
		if err != nil {
			return PartitionInfo{}, fmt.Errorf(
				"failed to partition NixOS config on disk %s: %w",
				nixosConfigDisk,
				err,
			)
		}
	}

	/*
	 --- ZFS Disks ---

	 Each ZFS disk is partitioned into a boot and data partition.

	 In a stripe or mirror configuration, the boot and data partitions are created on each disk.
	*/
	for index, zfsDisk := range configData.ZFS.Disks {
		bootPart, dataPart, err := partitionZFSDisk(execute, zfsDisk, index, configData)
		if err != nil {
			return PartitionInfo{}, fmt.Errorf(
				"failed to partition ZFS disk %s: %w",
				zfsDisk,
				err,
			)
		}
		partInfo.ZFSBoot = append(partInfo.ZFSBoot, bootPart)
		partInfo.ZFSData = append(partInfo.ZFSData, dataPart)
	}

	// Sleep briefly to allow the kernel to recognize new partitions
	if execute {
		log.Println("Waiting 5 seconds for partitions to settle...")
		time.Sleep(5 * time.Second)
	}

	log.Println("--- Disk Partitioning Complete ---")
	return partInfo, nil
}

// partitionUEFIDisk handles partitioning and formatting for the UEFI disk.
func partitionUEFIDisk(
	execute bool,
	configData *config.Config,
) (partitionNameUEFI string, err error) {
	partitionNumberUEFI := 1
	partitionNameUEFI = fmt.Sprintf("%s-part%d", configData.UEFI.Disk, partitionNumberUEFI)
	partitionSizeUEFI := configData.UEFI.Size

	log.Printf("Partitioning UEFI disk: %s\n", configData.UEFI.Disk)

	// Wipe the disk
	err = wipeDisk(execute, configData.UEFI.Disk)
	if err != nil {
		return "", fmt.Errorf("failed to wipe disk %s: %w", configData.UEFI.Disk, err)
	}

	// Create the UEFI partition with the specified size.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--new=%d:1M:%s", partitionNumberUEFI, partitionSizeUEFI),
		fmt.Sprintf("--typecode=%d:ef00", partitionNumberUEFI),
		fmt.Sprintf("--change-name=%d:uefi", partitionNumberUEFI),
		configData.UEFI.Disk,
	)
	if err != nil {
		return "", fmt.Errorf(
			"sgdisk --new (UEFI partition) failed for %s: %w",
			configData.UEFI.Disk,
			err,
		)
	}

	// Print the partition table.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--print=%s", configData.UEFI.Disk),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		log.Printf(
			"Warning: sgdisk --print failed for %s after partitioning: %v",
			configData.UEFI.Disk,
			err,
		)
	}

	log.Printf("Formatting UEFI partition: %s\n", partitionNameUEFI)

	// Format the UEFI partition
	err = formatPartition(
		execute,
		configData.UEFI.Disk,
		partitionNumberUEFI,
		partitionNameUEFI,
		"mkfs.fat",
		"-F", "32",
	)
	if err != nil {
		return "", fmt.Errorf("failed to format UEFI partition: %w", err)
	}

	return partitionNameUEFI, nil
}

// partitionNixOSConfigDisk handles partitioning and formatting for the NixOS config disk.
func partitionNixOSConfigDisk(
	execute bool,
	nixosConfigDisk string,
	uefiDisk string,
) (partitionNameNixOSConfig string, err error) {
	// If the NixOS config disk is the same as the UEFI disk, use the next partition number
	var partitionNumberNixOSConfig int
	if nixosConfigDisk == uefiDisk {
		partitionNumberNixOSConfig = 2 // UEFI is partition 1
	} else {
		partitionNumberNixOSConfig = 1
	}
	partitionNameNixOSConfig = fmt.Sprintf("%s-part%d", nixosConfigDisk, partitionNumberNixOSConfig)

	log.Printf(
		"Partitioning NixOS configuration on disk: %s using partition %d.\n",
		nixosConfigDisk,
		partitionNumberNixOSConfig,
	)

	// Create the NixOS config partition.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--new=%d:0:0", partitionNumberNixOSConfig),
		fmt.Sprintf("--typecode=%d:8300", partitionNumberNixOSConfig), // Linux filesystem
		fmt.Sprintf("--change-name=%d:nixos", partitionNumberNixOSConfig),
		nixosConfigDisk,
	)
	if err != nil {
		return "", fmt.Errorf(
			"sgdisk --new (NixOS config partition) failed for %s: %w",
			nixosConfigDisk,
			err,
		)
	}

	// Print the partition table.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--print=%s", nixosConfigDisk),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		log.Printf(
			"Warning: sgdisk --print failed for %s after partitioning: %v",
			nixosConfigDisk,
			err,
		)
	}

	log.Printf("Formatting NixOS config partition: %s\n", partitionNameNixOSConfig)

	// Format the NixOS config partition
	err = formatPartition(
		execute,
		nixosConfigDisk,
		partitionNumberNixOSConfig,
		partitionNameNixOSConfig,
		"mkfs.xfs",
		"-f",
		"-L",
		"nixos",
	)
	if err != nil {
		return "", fmt.Errorf("mkfs.xfs failed for %s: %w", partitionNameNixOSConfig, err)
	}

	return partitionNameNixOSConfig, nil
}

// partitionZFSDisk handles partitioning for a single ZFS disk.
// Returns ZFS boot and data partition names or an error.
func partitionZFSDisk(
	execute bool,
	zfsDisk string,
	index int,
	configData *config.Config,
) (partitionNameZFSBoot string, partitionNameZFSData string, err error) {
	partitionNumberZFSBoot := 1
	partitionNumberZFSData := 2
	partitionNameZFSBoot = fmt.Sprintf("%s-part%d", zfsDisk, partitionNumberZFSBoot)
	partitionNameZFSData = fmt.Sprintf("%s-part%d", zfsDisk, partitionNumberZFSData)

	log.Printf("Partitioning ZFS disk %d: %s\n", index+1, zfsDisk)

	// Wipe the disk
	err = wipeDisk(execute, zfsDisk)
	if err != nil {
		return "", "", fmt.Errorf("failed to wipe disk %s: %w", zfsDisk, err)
	}

	// Create the ZFS boot partition with the configured size.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf(
			"--new=%d:1M:%s",
			partitionNumberZFSBoot,
			configData.ZFS.BootPool.Size,
		),
		fmt.Sprintf("--typecode=%d:be00", partitionNumberZFSBoot),                   // Solaris Boot
		fmt.Sprintf("--change-name=%d:zfsboot-%d", partitionNumberZFSBoot, index+1), // Unique name
		zfsDisk,
	)
	if err != nil {
		return "", "", fmt.Errorf(
			"sgdisk --new (ZFS boot partition) failed for %s: %w",
			zfsDisk,
			err,
		)
	}

	// Create the ZFS root partition with all remaining space.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--new=%d:0:0", partitionNumberZFSData),
		fmt.Sprintf("--typecode=%d:bf00", partitionNumberZFSData),                   // Solaris Root
		fmt.Sprintf("--change-name=%d:zfsroot-%d", partitionNumberZFSData, index+1), // Unique name
		zfsDisk,
	)
	if err != nil {
		return "", "", fmt.Errorf(
			"sgdisk --new (ZFS root partition) failed for %s: %w",
			zfsDisk,
			err,
		)
	}

	// Print the partition table.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--print=%s", zfsDisk),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		log.Printf(
			"Warning: sgdisk --print failed for ZFS disk %s after partitioning: %v",
			zfsDisk,
			err,
		)
	}

	return partitionNameZFSBoot, partitionNameZFSData, nil
}

// getZFSDiskIDs finds the /dev/disk/by-id/ paths for the ZFS data partitions.
func getZFSDiskIDs(execute bool, zfsDisks []string) (zfsDiskIDs []string, err error) {
	log.Println("--- Retrieving ZFS Disk IDs ---")

	zfsDiskIDs = make([]string, len(zfsDisks))

	// For each disk, find the corresponding /dev/disk/by-id/ path
	for index, zfsDisk := range zfsDisks {
		log.Printf("Finding disk ID for ZFS disk %d: %s", index+1, zfsDisk)

		// Step 1: Determine base device name
		baseDevicePath := resolveDevicePath(execute, zfsDisk)
		baseDeviceName := path.Base(baseDevicePath)
		log.Printf("Resolved base device: %s", baseDevicePath)

		// Step 2: Determine device type to know the partition naming pattern
		isNVMe := strings.Contains(baseDeviceName, "nvme")

		// Sometimes /dev/disk/by-id takes a moment to update
		// so we need to retry a few times with different patterns
		var diskID string
		for attempt := 0; attempt < 5; attempt++ {
			// Try all possible naming patterns
			patterns := []string{}

			if isNVMe {
				// NVMe devices typically use -partN suffix in by-id paths
				patterns = append(patterns,
					fmt.Sprintf("*%s-part2", path.Base(zfsDisk)),
					fmt.Sprintf("*%s*p2", baseDeviceName),
					fmt.Sprintf("*%s*-part2", baseDeviceName))
			} else {
				// Traditional disks might use different patterns
				patterns = append(patterns,
					fmt.Sprintf("*%s2", path.Base(zfsDisk)),
					fmt.Sprintf("*%s*2", baseDeviceName),
					fmt.Sprintf("*%s*-part2", baseDeviceName))
			}

			// Try each pattern
			for _, pattern := range patterns {
				output, execErr := utils.Execute(
					execute,
					utils.ModeStdOut,
					"find",
					"/dev/disk/by-id/",
					"-lname",
					pattern,
				)

				if execErr == nil {
					// Clean up and check if we got a result
					possibleIDs := strings.Split(strings.TrimSpace(output), "\n")
					if len(possibleIDs) > 0 && possibleIDs[0] != "" {
						diskID = possibleIDs[0]
						log.Printf("Found disk ID using pattern '%s': %s", pattern, diskID)
						break
					}
				}
			}

			if diskID != "" {
				break // Found an ID, exit the retry loop
			}

			// If we can verify the partition exists directly, we can create a fallback
			// This handles cases where by-id links haven't been created yet
			if execute && attempt == 3 {
				// Force a global partition table update
				_, _ = utils.Execute(execute, utils.ModeNormal, "partprobe")
				_, _ = utils.Execute(execute, utils.ModeNormal, "udevadm", "trigger")
				_, _ = utils.Execute(execute, utils.ModeNormal, "udevadm", "settle")
			}

			log.Printf("Waiting for disk ID for %s (attempt %d of 5)...", zfsDisk, attempt+1)
			time.Sleep(2 * time.Second)
		}

		// Step 3: If we still can't find a proper by-id path, use a direct device path as fallback
		if diskID == "" {
			// Create a fallback ID based on the direct device path
			if isNVMe {
				// For NVMe try to build a partition path directly
				if strings.Contains(baseDevicePath, "nvme") {
					directPath := fmt.Sprintf("%sp2", baseDevicePath) // nvme0n1 -> nvme0n1p2
					log.Printf("Using direct NVMe partition path as fallback: %s", directPath)
					diskID = directPath
				}
			} else {
				// For traditional devices
				directPath := fmt.Sprintf("%s2", baseDevicePath) // sda -> sda2
				log.Printf("Using direct partition path as fallback: %s", directPath)
				diskID = directPath
			}
		}

		// Final check - do we have a valid ID?
		if diskID == "" {
			return nil, fmt.Errorf(
				"could not find any valid device ID for ZFS data partition on %s after multiple attempts",
				zfsDisk,
			)
		}

		zfsDiskIDs[index] = diskID
		log.Printf("Final ZFS data disk %d ID: %s", index+1, diskID)
	}

	log.Println("--- ZFS Disk ID Retrieval Complete ---")
	return zfsDiskIDs, nil
}

// wipeDisk thoroughly wipes a disk's partition tables and filesystem signatures.
// It uses multiple methods to ensure the disk is completely clean before partitioning.
func wipeDisk(execute bool, diskPath string) error {
	log.Printf("Wiping disk completely: %s\n", diskPath)

	// First try to clean ZFS-specific issues
	err := cleanZFSDisk(execute, diskPath)
	if err != nil {
		log.Printf(
			"Warning: Failed to clean disk using ZFS, trying fallback options for %s: %v",
			diskPath,
			err,
		)
	}

	// Get the real device path
	resolvedPath, err := utils.Execute(
		execute,
		utils.ModeStdOut,
		"readlink",
		"-f",
		diskPath,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve path for %s: %w", diskPath, err)
	}
	resolvedPath = strings.TrimSpace(resolvedPath)
	log.Printf("Resolved %s to %s", diskPath, resolvedPath)

	// After ZFS-specific cleanup, try the standard disk wiping methods

	// Try to wipe with sgdisk
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		"--zap-all",
		diskPath,
	)
	if err != nil {
		log.Printf("Warning: sgdisk --zap-all failed for %s: %v", diskPath, err)
	}

	// Try to wipe with wipefs
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"wipefs",
		"--all",
		diskPath,
	)
	if err != nil {
		log.Printf("Warning: wipefs failed, trying another method for %s: %v", diskPath, err)

		// Use dd to wipe the beginning of the disk
		_, err = utils.Execute(
			execute,
			utils.ModeNormal,
			"dd",
			"if=/dev/zero",
			"of="+diskPath,
			"bs=1M",
			"count=32", // Increased from 8 to 32MB
			"conv=fsync",
		)
		if err != nil {
			log.Printf("Warning: dd zeroing failed for %s: %v", diskPath, err)
		}
	}

	// Give a moment for kernel to recognize changes
	time.Sleep(2 * time.Second)

	// Use sgdisk to create a new GPT
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		"--clear",
		diskPath,
	)
	if err != nil {
		log.Printf("Warning: sgdisk --clear failed for %s: %v", diskPath, err)
		return fmt.Errorf("failed to clear partition table on %s: %w", diskPath, err)
	}

	// Wait for changes to be recognized by the system
	log.Println("Waiting for disk changes to be recognized...")
	time.Sleep(5 * time.Second)

	return nil
}

// resolveDevicePath resolves a possibly symlinked device path (especially from /dev/disk/by-id/)
// to its actual /dev path (e.g., /dev/sda)
func resolveDevicePath(execute bool, diskPath string) string {
	// If it's not a by-id path, just return it
	if !strings.Contains(diskPath, "/dev/disk/by-id/") {
		return diskPath
	}

	// Get the actual device the symlink points to
	deviceOutput, err := utils.Execute(
		execute,
		utils.ModeStdOut,
		"readlink",
		"-f",
		diskPath,
	)
	if err == nil {
		resolvedPath := strings.TrimSpace(deviceOutput)
		log.Printf("Resolved %s to %s", diskPath, resolvedPath)
		return resolvedPath
	}

	log.Printf("Warning: could not resolve symlink %s: %v", diskPath, err)
	return diskPath // Return original if resolution fails
}

// findPartitionDevicePath tries multiple strategies to find a usable device path for a partition
// It returns the first working path or an empty string if no path was found
func findPartitionDevicePath(
	execute bool,
	diskPath string,
	partitionNumber int,
	originalPartitionPath string,
) string {
	if !execute {
		return originalPartitionPath // In dry-run mode, just return the original path
	}

	// Try to find the direct device path first (e.g., /dev/sdb1)
	baseDiskPath := resolveDevicePath(execute, diskPath)

	// Extract the base device name and construct direct partition path
	if strings.HasPrefix(baseDiskPath, "/dev/") {
		baseDiskName := path.Base(baseDiskPath)
		directPartitionPath := fmt.Sprintf("/dev/%s%d", baseDiskName, partitionNumber)
		log.Printf("Checking for direct device path: %s", directPartitionPath)

		// Check multiple times for the direct path
		for attempt := 1; attempt <= 5; attempt++ {
			if _, err := os.Stat(directPartitionPath); err == nil {
				log.Printf("Found direct partition path: %s", directPartitionPath)
				return directPartitionPath
			}

			_, _ = utils.Execute(execute, utils.ModeNormal, "partprobe", baseDiskPath)
			log.Printf("Waiting for direct device path (attempt %d of 5)...", attempt)
			time.Sleep(2 * time.Second)
		}
	}

	// If direct path failed, try the by-id path
	log.Printf("Checking for by-id device path: %s", originalPartitionPath)
	for attempt := 1; attempt <= 5; attempt++ {
		if _, err := os.Stat(originalPartitionPath); err == nil {
			log.Printf("Found by-id partition path: %s", originalPartitionPath)
			return originalPartitionPath
		}

		_, _ = utils.Execute(execute, utils.ModeNormal, "partprobe", diskPath)
		log.Printf("Waiting for by-id device path (attempt %d of 5)...", attempt)
		time.Sleep(2 * time.Second)
	}

	// Last resort - run partprobe globally
	log.Printf("Running partprobe globally to update all partition tables...")
	_, _ = utils.Execute(execute, utils.ModeNormal, "partprobe")
	time.Sleep(3 * time.Second)

	// One last check for direct path
	if strings.HasPrefix(baseDiskPath, "/dev/") {
		baseDiskName := path.Base(baseDiskPath)
		directPartitionPath := fmt.Sprintf("/dev/%s%d", baseDiskName, partitionNumber)
		if _, err := os.Stat(directPartitionPath); err == nil {
			log.Printf(
				"Found direct partition path after global partprobe: %s",
				directPartitionPath,
			)
			return directPartitionPath
		}
	}

	// If we still don't have a path, warn but return empty string
	log.Printf(
		"Warning: Could not find a usable device path for partition %d on %s",
		partitionNumber,
		diskPath,
	)
	return ""
}

// formatPartition formats a partition, ensuring a usable device path is found first
// Returns an error if formatting fails
func formatPartition(
	execute bool,
	diskPath string,
	partitionNumber int,
	partitionNameOriginal string,
	formatCmd string,
	formatArgs ...string,
) error {
	if execute {
		log.Printf("Waiting for partition device to be available: %s\n", partitionNameOriginal)

		// Try to find a usable device path
		devicePath := findPartitionDevicePath(
			execute,
			diskPath,
			partitionNumber,
			partitionNameOriginal,
		)

		// If no path was found, fall back to the original
		if devicePath == "" {
			log.Printf("Warning: Using original path as fallback: %s", partitionNameOriginal)
			devicePath = partitionNameOriginal
		}

		log.Printf("Formatting partition using device path: %s\n", devicePath)

		// Prepare command arguments
		allArgs := append([]string{formatCmd}, formatArgs...)
		allArgs = append(allArgs, devicePath)

		// Execute the format command
		_, err := utils.Execute(
			execute,
			utils.ModeNormal,
			allArgs[0],
			allArgs[1:]...,
		)

		if err != nil {
			return fmt.Errorf("%s failed for %s: %w", formatCmd, devicePath, err)
		}
	} else {
		// In dry-run mode, log what would happen
		allArgs := append([]string{formatCmd}, formatArgs...)
		allArgs = append(allArgs, partitionNameOriginal)
		log.Printf("Would format partition: %s using command: %s", partitionNameOriginal, strings.Join(allArgs, " "))
	}

	return nil
}

// cleanZFSDisk properly handles ZFS disk cleaning
func cleanZFSDisk(execute bool, diskPath string) error {
	log.Printf("Performing thorough ZFS cleanup for disk: %s", diskPath)

	// Step 1: Try to offline the disk from any active pools
	poolInfo, err := utils.Execute(
		execute,
		utils.ModeStdOut,
		"zpool",
		"status",
		"-P", // Physical path
	)

	if err == nil && poolInfo != "" {
		// Parse the output to find if disk is part of any pools
		diskPaths, err := utils.Execute(
			execute,
			utils.ModeStdOut,
			"readlink",
			"-f",
			diskPath,
		)
		if err == nil {
			resolvedPath := strings.TrimSpace(diskPaths)
			baseDiskName := path.Base(resolvedPath)

			// Check if disk appears in any pools
			for _, poolLine := range strings.Split(poolInfo, "\n") {
				if strings.Contains(poolLine, baseDiskName) {
					// Extract pool name from the output (assuming format like "pool: zpool")
					var poolName string
					for _, line := range strings.Split(poolInfo, "\n") {
						if strings.HasPrefix(line, "pool:") {
							poolName = strings.TrimSpace(strings.TrimPrefix(line, "pool:"))
							break
						}
					}

					if poolName != "" {
						log.Printf(
							"Found disk %s in pool %s, trying to offline",
							baseDiskName,
							poolName,
						)

						// Try to offline the disk
						_, err = utils.Execute(
							execute,
							utils.ModeNormal,
							"zpool",
							"offline",
							poolName,
							resolvedPath,
						)
						if err != nil {
							log.Printf(
								"Warning: Could not offline disk %s from pool %s: %v",
								baseDiskName,
								poolName,
								err,
							)
						}

						// Try to export the pool
						_, err = utils.Execute(
							execute,
							utils.ModeNormal,
							"zpool",
							"export",
							"-f",
							poolName,
						)
						if err != nil {
							log.Printf("Warning: Could not export pool %s: %v", poolName, err)
						}
					}
					break
				}
			}
		}
	}

	// Step 2: Try the ZFS labelclear command
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zpool",
		"labelclear",
		"-f",
		diskPath,
	)
	if err != nil {
		log.Printf("Warning: zpool labelclear failed for %s: %v", diskPath, err)
	}

	// Step 3: Force destroy any remaining pools on this disk
	// This is a more aggressive approach
	_, err = utils.Execute(
		execute,
		utils.ModeStdOut,
		"zpool",
		"import",
		"-d",
		path.Dir(diskPath),
	)
	if err == nil {
		// If there are pools, try to destroy them
		_, err = utils.Execute(
			execute,
			utils.ModeNormal,
			"zpool",
			"import",
			"-d",
			path.Dir(diskPath),
			"-f",
			"-N",
			"-a",
		)
		if err != nil {
			log.Printf("Warning: Failed to import pools for destruction from %s: %v", diskPath, err)
		} else {
			// Get pool names
			poolNames, err := utils.Execute(
				execute,
				utils.ModeStdOut,
				"zpool",
				"list",
				"-H",
				"-o",
				"name",
			)
			if err == nil {
				for _, pool := range strings.Split(strings.TrimSpace(poolNames), "\n") {
					if pool != "" {
						log.Printf("Destroying imported pool: %s", pool)
						_, err = utils.Execute(
							execute,
							utils.ModeNormal,
							"zpool",
							"destroy",
							"-f",
							pool,
						)
						if err != nil {
							log.Printf("Warning: Failed to destroy pool %s: %v", pool, err)
						}
					}
				}
			}
		}
	}

	return nil
}

// forceUnmountMountpoints unmounts all mountpoints in the /mnt/nixos directory using Linux kernel's force unmount
func forceUnmountMountpoints(execute bool) error {
	// Get all mounted filesystems from /proc/mounts
	mountsOutput, err := utils.Execute(
		execute,
		utils.ModeStdOut,
		"grep",
		mountPoint,
		"/proc/mounts",
	)

	if err == nil && mountsOutput != "" {
		// Split into lines and process in reverse order (unmount deepest paths first)
		mounts := strings.Split(strings.TrimSpace(mountsOutput), "\n")
		sort.Sort(sort.Reverse(sort.StringSlice(mounts)))

		for _, mount := range mounts {
			fields := strings.Fields(mount)
			if len(fields) >= 2 {
				mountpointFound := fields[1]
				if strings.HasPrefix(mountpointFound, mountPoint) {
					log.Printf("Force unmounting: %s", mountpointFound)
					// Use Linux umount with force and detach options
					_, err := utils.Execute(
						execute,
						utils.ModeNormal,
						"umount",
						"-f",
						"-l",
						mountpointFound,
					)
					if err != nil {
						log.Printf("Warning: Failed to force unmount %s: %v", mountpointFound, err)
					}
				}
			}
		}
	}

	return nil
}
