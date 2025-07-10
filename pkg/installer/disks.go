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

const (
	shortWaitTime  = 2 * time.Second
	mediumWaitTime = 3 * time.Second
	longWaitTime   = 5 * time.Second
	maxAttempts    = 5
	minFieldCount  = 2
)

// PartitionInfo holds information about created partitions
type PartitionInfo struct {
	UEFI        string   // UEFI/ESP partition
	NixOSConfig string   // NixOS configuration partition
	ZFSPool     []string // ZFS pool disks
}

//nolint:gocyclo // unmountDisks unmounts all partitions on the specified disks in the config.
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
	var allZFSDisks []string
	allZFSDisks = append(allZFSDisks, configData.ZFS.Pool.Disks.Cache...)
	allZFSDisks = append(allZFSDisks, configData.ZFS.Pool.Disks.Log...)
	allZFSDisks = append(allZFSDisks, configData.ZFS.Pool.Disks.Data...)
	allZFSDisks = append(allZFSDisks, configData.ZFS.Pool.Disks.Spare...)

	for _, zfsDisk := range allZFSDisks {

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

// wipeAndPartitionDisks handles wiping and partitioning the specified disks according to the configuration.
// Returns partition info and an error if any step fails.
func wipeAndPartitionDisks(
	execute bool,
	configData *config.Config,
) (partitionInfo PartitionInfo, err error) {
	partInfo := PartitionInfo{
		UEFI:        "",
		NixOSConfig: "",
		ZFSPool:     []string{},
	}

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

	 Each ZFS disk is wiped and partitioned in preparation for adding to the pool.
	*/

	// Wipe and partition any cache disks.
	for _, cacheDisk := range configData.ZFS.Pool.Disks.Cache {
		err = wipeDisk(execute, cacheDisk)
		if err != nil {
			return PartitionInfo{}, fmt.Errorf("failed to wipe cache disk %s: %w", cacheDisk, err)
		}
		err = partitionZFSDisk(execute, cacheDisk, "cache")
		if err != nil {
			return PartitionInfo{}, fmt.Errorf(
				"failed to partition cache disk %s: %w",
				cacheDisk,
				err,
			)
		}
	}

	// Wipe and partition any log disks.
	for _, logDisk := range configData.ZFS.Pool.Disks.Log {
		err = wipeDisk(execute, logDisk)
		if err != nil {
			return PartitionInfo{}, fmt.Errorf("failed to wipe log disk %s: %w", logDisk, err)
		}
		err = partitionZFSDisk(execute, logDisk, "log")
		if err != nil {
			return PartitionInfo{}, fmt.Errorf("failed to partition log disk %s: %w", logDisk, err)
		}
	}

	// Wipe and partition any data disks.
	for _, dataDisk := range configData.ZFS.Pool.Disks.Data {
		err = wipeDisk(execute, dataDisk)
		if err != nil {
			return PartitionInfo{}, fmt.Errorf("failed to wipe data disk %s: %w", dataDisk, err)
		}
		err = partitionZFSDisk(execute, dataDisk, "data")
		if err != nil {
			return PartitionInfo{}, fmt.Errorf(
				"failed to partition data disk %s: %w",
				dataDisk,
				err,
			)
		}
	}

	// Wipe and partition any spare disks.
	for _, spareDisk := range configData.ZFS.Pool.Disks.Spare {
		err = wipeDisk(execute, spareDisk)
		if err != nil {
			return PartitionInfo{}, fmt.Errorf("failed to wipe spare disk %s: %w", spareDisk, err)
		}
		err = partitionZFSDisk(execute, spareDisk, "spare")
		if err != nil {
			return PartitionInfo{}, fmt.Errorf(
				"failed to partition spare disk %s: %w",
				spareDisk,
				err,
			)
		}
	}

	log.Println("--- Disk Wiping and Partitioning Complete ---")
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

// partitionZFSDisk creates a single partition on a ZFS disk for use in the pool.
func partitionZFSDisk(execute bool, diskPath string, diskType string) error {
	log.Printf("Partitioning ZFS %s disk: %s", diskType, diskPath)

	// Create a single partition that uses the entire disk
	_, err := utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		"--new=1:0:0",
		"--typecode=1:bf00", // Solaris partition type
		fmt.Sprintf("--change-name=1:zfs-%s", diskType),
		diskPath,
	)
	if err != nil {
		return fmt.Errorf("failed to create partition on %s disk %s: %w", diskType, diskPath, err)
	}

	// Print the partition table for verification
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--print=%s", diskPath),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		log.Printf(
			"Warning: sgdisk --print failed for %s disk %s after partitioning: %v",
			diskType,
			diskPath,
			err,
		)
	}

	return nil
}

// findDiskIDByID finds the canonical /dev/disk/by-id path for a single disk.
// It resolves the input path to a base device and searches for a matching by-id link.
func findDiskIDByID(execute bool, diskPath string) (string, error) {
	log.Printf("Finding by-id path for disk: %s", diskPath)

	// 1. Resolve input to base device path (e.g., /dev/sda, /dev/nvme0n1)
	// Always execute readlink to resolve the path, even in dry-run, for accuracy.
	baseDevicePathOutput, err := utils.Execute(true, utils.ModeStdOut, "readlink", "-f", diskPath)
	if err != nil {
		// If readlink fails, maybe the path is already the base path? Check if it exists.
		if _, statErr := os.Stat(diskPath); statErr == nil {
			log.Printf(
				"Warning: readlink failed for %s (%v), assuming it's already the base path.",
				diskPath,
				err,
			)
			baseDevicePathOutput = diskPath // Use the input path directly
		} else {
			return "", fmt.Errorf("failed to resolve base device path for %s: %w", diskPath, err)
		}
	}
	baseDevicePath := strings.TrimSpace(baseDevicePathOutput)
	log.Printf("Resolved %s to base device path: %s", diskPath, baseDevicePath)

	// 2. Retry finding the matching by-id link
	const maxAttempts = 5
	const retryDelay = 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		entries, err := os.ReadDir("/dev/disk/by-id")
		if err != nil {
			// This directory should generally exist
			return "", fmt.Errorf("failed to read /dev/disk/by-id: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue // Skip directories
			}
			byIDPath := path.Join("/dev/disk/by-id", entry.Name())

			// Resolve the by-id path symlink
			// Always execute readlink to check the link target.
			resolvedLinkOutput, err := utils.Execute(
				true,
				utils.ModeStdOut,
				"readlink",
				"-f",
				byIDPath,
			)
			if err != nil {
				// Log warning but continue checking other links
				log.Printf("Warning: could not resolve symlink %s: %v", byIDPath, err)
				continue
			}
			resolvedLink := strings.TrimSpace(resolvedLinkOutput)

			// Check if it matches the base device path *exactly*
			if resolvedLink == baseDevicePath {
				log.Printf("Found matching by-id path: %s -> %s", byIDPath, resolvedLink)
				return byIDPath, nil // Success!
			}
		}

		// If not found, wait and maybe trigger udev
		if attempt < maxAttempts {
			log.Printf(
				"Matching by-id path for %s not found (attempt %d/%d). Waiting...",
				baseDevicePath,
				attempt,
				maxAttempts,
			)
			if execute {
				// Trigger udev updates, errors are warnings
				_, err = utils.Execute(
					execute,
					utils.ModeSilent,
					"udevadm",
					"settle",
					"--timeout=5",
				)
				if err != nil {
					log.Printf("Warning: udevadm settle failed: %v", err)
				}
			}
			time.Sleep(retryDelay)
		}
	}

	// If still not found after retries
	log.Printf(
		"Error: Could not find a /dev/disk/by-id/ link pointing to %s after %d attempts.",
		baseDevicePath,
		maxAttempts,
	)
	return "", fmt.Errorf(
		"no /dev/disk/by-id/ link found for %s (resolved to %s)",
		diskPath,
		baseDevicePath,
	)
}

// getDiskIDsByID finds the canonical /dev/disk/by-id/ paths for the given disk paths.
// It replaces the old getZFSDiskIDs function.
// If the provided diskPaths is empty, will return an empty string.
func getDiskIDsByID(execute bool, diskPaths []string) ([]string, error) {
	log.Println("--- Retrieving Disk IDs by /dev/disk/by-id ---")
	diskIDs := make([]string, len(diskPaths))
	var errorsCollected []error

	for i, p := range diskPaths {
		diskID, err := findDiskIDByID(execute, p)
		if err != nil {
			log.Printf("Error finding ID for disk %s: %v", p, err)
			// Collect errors to report all failures at the end
			errorsCollected = append(errorsCollected, fmt.Errorf("disk '%s': %w", p, err))
			diskIDs[i] = "" // Indicate failure for this disk
		} else {
			diskIDs[i] = diskID
			log.Printf("Successfully found ID for disk %d (%s): %s", i+1, p, diskID)
		}
	}

	log.Println("--- Disk ID Retrieval Complete ---")
	if len(errorsCollected) > 0 {
		// Combine errors into a single error message
		errorStrings := make([]string, len(errorsCollected))
		for i, e := range errorsCollected {
			errorStrings[i] = e.Error()
		}
		return diskIDs, fmt.Errorf(
			"failed to retrieve some disk IDs:\n - %s",
			strings.Join(errorStrings, "\n - "),
		)
	}

	return diskIDs, nil
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

	// Get the real device path, which handles dry run mode properly
	resolvedPath := resolveDevicePath(execute, diskPath)

	// After ZFS-specific cleanup, try the standard disk wiping methods
	// Try to wipe with sgdisk
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		"--zap-all",
		resolvedPath,
	)
	if err != nil {
		log.Printf("Warning: sgdisk --zap-all failed for %s: %v", resolvedPath, err)
	}

	// Try to wipe with wipefs
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"wipefs",
		"--all",
		resolvedPath,
	)
	if err != nil {
		log.Printf("Warning: wipefs failed, trying another method for %s: %v", resolvedPath, err)

		// Use dd to wipe the beginning of the disk
		_, err = utils.Execute(
			execute,
			utils.ModeNormal,
			"dd",
			"if=/dev/zero",
			"of="+resolvedPath,
			"bs=1M",
			"count=32",
			"conv=fsync",
		)
		if err != nil {
			log.Printf("Warning: dd zeroing failed for %s: %v", resolvedPath, err)
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
		resolvedPath,
	)
	if err != nil {
		log.Printf("Warning: sgdisk --clear failed for %s: %v", resolvedPath, err)
		return fmt.Errorf("failed to clear partition table on %s: %w", resolvedPath, err)
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

	// Handle dry-run mode first
	if !execute {
		// In dry-run mode, simulate the resolution based on disk type
		if strings.Contains(diskPath, "nvme") {
			log.Printf("Resolved %s to base device path: /dev/nvme0n1 (dry-run mode)", diskPath)
			return "/dev/nvme0n1"
		}
		log.Printf("Resolved %s to base device path: /dev/sda (dry-run mode)", diskPath)
		return "/dev/sda"
	}

	// Get the actual device the symlink points to (only in real execution mode)
	deviceOutput, err := utils.Execute(
		execute,
		utils.ModeStdOut,
		"readlink",
		"-f",
		diskPath,
	)
	if err == nil {
		resolvedPath := strings.TrimSpace(deviceOutput)
		if resolvedPath != "" {
			log.Printf("Resolved %s to %s", diskPath, resolvedPath)
			return resolvedPath
		}
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

			_, err := utils.Execute(execute, utils.ModeSilent, "partprobe", baseDiskPath)
			if err != nil {
				log.Printf("Warning: partprobe failed for %s: %v", baseDiskPath, err)
			}
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

		_, err := utils.Execute(execute, utils.ModeSilent, "partprobe", diskPath)
		if err != nil {
			log.Printf("Warning: partprobe failed for %s: %v", diskPath, err)
		}
		log.Printf("Waiting for by-id device path (attempt %d of 5)...", attempt)
		time.Sleep(2 * time.Second)
	}

	// Last resort - run partprobe globally
	log.Printf("Running partprobe globally to update all partition tables...")
	_, err := utils.Execute(execute, utils.ModeSilent, "partprobe")
	if err != nil {
		log.Printf("Warning: partprobe failed: %v", err)
	}
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

//nolint:gocyclo // cleanZFSDisk properly handles ZFS disk cleaning
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
			if len(fields) >= minFieldCount {
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
