package installer

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
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

	sysutil.Info("--- Unmounting Disks ---")

	// First try to unmount everything using Linux kernel's force unmount
	// This is more reliable than ZFS's unmount for busy filesystems
	err = forceUnmountMountpoints(execute)
	if err != nil {
		// Non-fatal error, just log it
		sysutil.Warn("Failed to force unmount mountpoints: %v", err)
	}

	sysutil.Info("Processing ZFS Pools")

	// First get a list of all pools
	poolsOutput, err := sysutil.Execute(
		execute,
		sysutil.ModeStdOut,
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

			sysutil.Info("Unmounting all datasets in pool: %s", pool)

			// First list all datasets in reverse order (children first)
			datasetsOutput, listErr := sysutil.Execute(
				execute,
				sysutil.ModeStdOut,
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
					sysutil.Info("Unmounting dataset: %s", dataset)
					stderr, unmountErr := sysutil.Execute(
						execute,
						sysutil.ModeStdErr,
						"zfs",
						"unmount",
						dataset,
					)
					if unmountErr != nil {
						if stderr != "" {
							sysutil.Warn(
								"Failed to unmount dataset %s: %s",
								dataset,
								strings.TrimSpace(stderr),
							)
						} else {
							sysutil.Warn("Failed to unmount dataset %s: %v", dataset, unmountErr)
						}
						// Try force unmount if regular unmount fails
						forceStderr, forceErr := sysutil.Execute(
							execute,
							sysutil.ModeStdErr,
							"zfs",
							"unmount",
							"-f",
							dataset,
						)
						if forceErr != nil {
							if forceStderr != "" {
								sysutil.Warn(
									"Force unmount also failed for %s: %s",
									dataset,
									strings.TrimSpace(forceStderr),
								)
							} else {
								sysutil.Warn("Force unmount also failed for %s: %v", dataset, forceErr)
							}
						}
					}
				}
			}

			// Now export the pool
			sysutil.Info("Exporting pool: %s", pool)
			_, exportErr := sysutil.Execute(
				execute,
				sysutil.ModeNormal,
				"zpool",
				"export",
				"-f",
				pool,
			)
			if exportErr != nil {
				sysutil.Warn("Failed to export pool %s: %v", pool, exportErr)
			}
		}
	} else {
		// Fallback to the original approach
		_, err = sysutil.Execute(execute, sysutil.ModeNormal, "zfs", "unmount", "-a", "-f")
		if err != nil {
			sysutil.Warn("Failed to unmount ZFS pools: %v", err)
		}
		time.Sleep(3 * time.Second) // Give time for unmounts to finish

		// Process ZFS export
		_, err = sysutil.Execute(execute, sysutil.ModeNormal, "zpool", "export", "-a", "-f")
		if err != nil {
			sysutil.Warn("Failed to export ZFS pools: %v", err)
		}
		time.Sleep(3 * time.Second) // Give time for exports to finish
	}

	sysutil.Info("Processing other mounts")

	// Create a slice to store all mountpoints
	var allMountPoints []string

	// Get all block devices as JSON
	// Need to always run this even in dry-run mode
	blockDevicesJSON, err := sysutil.Execute(
		true,
		sysutil.ModeStdOut,
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

		sysutil.Info("Checking for mountpoints on UEFI disk: %s", configData.UEFI.Disk)

		uefiMountPoints, err := sysutil.GetMountpoints(
			configData.UEFI.Disk,
			[]byte(blockDevicesJSON),
		)
		if err != nil {
			return fmt.Errorf(
				"failed to get mountpoints for UEFI disk %s: %w",
				configData.UEFI.Disk,
				err,
			)
		}

		// Add UEFI mountpoints to our list
		for _, mp := range uefiMountPoints {
			sysutil.Info("Found mountpoint on UEFI disk: %s", mp)
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

		sysutil.Info("Checking for mountpoints on ZFS disk: %s", zfsDisk)

		zfsMountPoints, err := sysutil.GetMountpoints(zfsDisk, []byte(blockDevicesJSON))
		if err != nil {
			return fmt.Errorf("failed to get mountpoints for ZFS disk %s: %w", zfsDisk, err)
		}

		// Add ZFS disk mountpoints to our list
		for _, mp := range zfsMountPoints {
			sysutil.Info("Found mountpoint on ZFS disk: %s", mp)
			allMountPoints = append(allMountPoints, mp)
		}
	}

	// If mountpoints have been found, unmount them
	if len(allMountPoints) > 0 {
		sysutil.Info("Found %d mountpoints to unmount", len(allMountPoints))
		err = sysutil.UnmountAll(execute, allMountPoints)
		if err != nil {
			return fmt.Errorf("failed to unmount all mountpoints: %w", err)
		}
	} else {
		sysutil.Info("No mountpoints found, nothing to unmount")
	}

	sysutil.Info("--- Unmounting Disks Complete ---")
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

	sysutil.Info("--- Partitioning Disks ---")

	// Process UEFI disk
	if err := processUEFIDisk(execute, configData, &partInfo); err != nil {
		return PartitionInfo{}, err
	}

	// Process NixOS config disk if enabled
	if err := processNixOSConfigDisk(execute, configData, &partInfo); err != nil {
		return PartitionInfo{}, err
	}

	// Process all ZFS disks
	if err := processZFSDisks(execute, configData); err != nil {
		return PartitionInfo{}, err
	}

	sysutil.Info("--- Disk Wiping and Partitioning Complete ---")
	return partInfo, nil
}

// processUEFIDisk handles UEFI disk partitioning
func processUEFIDisk(execute bool, configData *config.Config, partInfo *PartitionInfo) error {
	uefiDiskConfig := configData.UEFI
	var err error
	partInfo.UEFI, err = partitionUEFIDisk(execute, configData)
	if err != nil {
		return fmt.Errorf(
			"failed to partition UEFI disk %s: %w",
			uefiDiskConfig.Disk,
			err,
		)
	}
	return nil
}

// processNixOSConfigDisk handles NixOS config disk partitioning if enabled
func processNixOSConfigDisk(
	execute bool,
	configData *config.Config,
	partInfo *PartitionInfo,
) error {
	if !configData.NixOS.Config.Enabled {
		return nil
	}

	nixosConfigDisk := configData.UEFI.Disk
	uefiDiskConfig := configData.UEFI
	var err error
	partInfo.NixOSConfig, err = partitionNixOSConfigDisk(
		execute,
		nixosConfigDisk,
		uefiDiskConfig.Disk,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to partition NixOS config on disk %s: %w",
			nixosConfigDisk,
			err,
		)
	}
	return nil
}

// processZFSDisks handles wiping and partitioning all ZFS disks
func processZFSDisks(execute bool, configData *config.Config) error {
	// Process each type of ZFS disk
	if err := processZFSDiskType(execute, configData.ZFS.Pool.Disks.Cache, "cache"); err != nil {
		return err
	}
	if err := processZFSDiskType(execute, configData.ZFS.Pool.Disks.Log, "log"); err != nil {
		return err
	}
	if err := processZFSDiskType(execute, configData.ZFS.Pool.Disks.Data, "data"); err != nil {
		return err
	}
	if err := processZFSDiskType(execute, configData.ZFS.Pool.Disks.Spare, "spare"); err != nil {
		return err
	}
	return nil
}

// processZFSDiskType processes disks of a specific ZFS type (cache, log, data, spare)
func processZFSDiskType(execute bool, disks []string, diskType string) error {
	for _, disk := range disks {
		if err := wipeDisk(execute, disk); err != nil {
			return fmt.Errorf("failed to wipe %s disk %s: %w", diskType, disk, err)
		}
		if err := partitionZFSDisk(execute, disk, diskType); err != nil {
			return fmt.Errorf("failed to partition %s disk %s: %w", diskType, disk, err)
		}
	}
	return nil
}

// partitionUEFIDisk handles partitioning and formatting for the UEFI disk.
func partitionUEFIDisk(
	execute bool,
	configData *config.Config,
) (partitionNameUEFI string, err error) {
	partitionNumberUEFI := 1
	partitionNameUEFI = fmt.Sprintf("%s-part%d", configData.UEFI.Disk, partitionNumberUEFI)
	partitionSizeUEFI := configData.UEFI.Size

	sysutil.Info("Partitioning UEFI disk: %s", configData.UEFI.Disk)

	// Wipe the disk
	err = wipeDisk(execute, configData.UEFI.Disk)
	if err != nil {
		return "", fmt.Errorf("failed to wipe disk %s: %w", configData.UEFI.Disk, err)
	}

	// Create the UEFI partition with the specified size.
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
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
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--print=%s", configData.UEFI.Disk),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		sysutil.Warn(
			"sgdisk --print failed for %s after partitioning: %v",
			configData.UEFI.Disk,
			err,
		)
	}

	sysutil.Info("Formatting UEFI partition: %s", partitionNameUEFI)

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

	sysutil.Info(
		"Partitioning NixOS configuration on disk: %s using partition %d.",
		nixosConfigDisk,
		partitionNumberNixOSConfig,
	)

	// Create the NixOS config partition.
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
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
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--print=%s", nixosConfigDisk),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		sysutil.Warn(
			"sgdisk --print failed for %s after partitioning: %v",
			nixosConfigDisk,
			err,
		)
	}

	sysutil.Info("Formatting NixOS config partition: %s", partitionNameNixOSConfig)

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
	sysutil.Info("Partitioning ZFS %s disk: %s", diskType, diskPath)

	// Create a single partition that uses the entire disk
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
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
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--print=%s", diskPath),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		sysutil.Warn(
			"sgdisk --print failed for %s disk %s after partitioning: %v",
			diskType,
			diskPath,
			err,
		)
	}

	// After partitioning, we must wait for the partition device to appear.
	// This is crucial because 'zpool create' will fail if the device node doesn't exist yet.
	if execute {
		sysutil.Info("Waiting for partition on %s to become available...", diskPath)

		// The standard for /dev/disk/by-id partition links is to use the "-partN" suffix.
		expectedPartitionPath := fmt.Sprintf("%s-part1", diskPath)

		// Poll for the partition to exist.
		const maxAttempts = 10
		const retryDelay = 2 * time.Second
		var found bool
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			// Tell the kernel to re-read the partition table.
			// We do this in the loop in case it takes a moment to process.
			_, err := sysutil.Execute(true, sysutil.ModeSilent, "partprobe", diskPath)
			if err != nil {
				sysutil.Warn(
					"partprobe failed on attempt %d for %s: %v",
					attempt,
					diskPath,
					err,
				)
			}

			// Check if the partition file exists.
			if _, err := os.Stat(expectedPartitionPath); err == nil {
				sysutil.Info("Partition %s found after attempt %d.", expectedPartitionPath, attempt)
				found = true
				break // Success!
			}

			// If not found, wait before retrying.
			sysutil.Info(
				"Partition %s not yet found. Waiting... (attempt %d/%d)",
				expectedPartitionPath,
				attempt,
				maxAttempts,
			)
			time.Sleep(retryDelay)
		}

		if !found {
			// If we exit the loop without finding the partition, it's a fatal error.
			return fmt.Errorf(
				"partition %s did not appear after %d attempts",
				expectedPartitionPath,
				maxAttempts,
			)
		}
	}

	return nil
}

// findDiskIDByID finds the canonical /dev/disk/by-id path for a single disk.
// It resolves the input path to a base device and searches for a matching by-id link.
func findDiskIDByID(execute bool, diskPath string) (string, error) {
	sysutil.Info("Finding by-id path for disk: %s", diskPath)

	// 1. Resolve input to base device path (e.g., /dev/sda, /dev/nvme0n1)
	// Always execute readlink to resolve the path, even in dry-run, for accuracy.
	baseDevicePathOutput, err := sysutil.Execute(
		true,
		sysutil.ModeStdOut,
		"readlink",
		"-f",
		diskPath,
	)
	if err != nil {
		// If readlink fails, maybe the path is already the base path? Check if it exists.
		if _, statErr := os.Stat(diskPath); statErr == nil {
			sysutil.Warn(
				"readlink failed for %s (%v), assuming it's already the base path.",
				diskPath,
				err,
			)
			baseDevicePathOutput = diskPath // Use the input path directly
		} else {
			return "", fmt.Errorf("failed to resolve base device path for %s: %w", diskPath, err)
		}
	}
	baseDevicePath := strings.TrimSpace(baseDevicePathOutput)
	sysutil.Info("Resolved %s to base device path: %s", diskPath, baseDevicePath)

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
			resolvedLinkOutput, err := sysutil.Execute(
				true,
				sysutil.ModeStdOut,
				"readlink",
				"-f",
				byIDPath,
			)
			if err != nil {
				// Log warning but continue checking other links
				sysutil.Warn("could not resolve symlink %s: %v", byIDPath, err)
				continue
			}
			resolvedLink := strings.TrimSpace(resolvedLinkOutput)

			// Check if it matches the base device path *exactly*
			if resolvedLink == baseDevicePath {
				sysutil.Info("Found matching by-id path: %s -> %s", byIDPath, resolvedLink)
				return byIDPath, nil // Success!
			}
		}

		// If not found, wait and maybe trigger udev
		if attempt < maxAttempts {
			sysutil.Info(
				"Matching by-id path for %s not found (attempt %d/%d). Waiting...",
				baseDevicePath,
				attempt,
				maxAttempts,
			)
			if execute {
				// Trigger udev updates, errors are warnings
				_, err = sysutil.Execute(
					execute,
					sysutil.ModeSilent,
					"udevadm",
					"settle",
					"--timeout=5",
				)
				if err != nil {
					sysutil.Warn("udevadm settle failed: %v", err)
				}
			}
			time.Sleep(retryDelay)
		}
	}

	// If still not found after retries
	sysutil.Error(
		"Could not find a /dev/disk/by-id/ link pointing to %s after %d attempts.",
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
	sysutil.Info("--- Retrieving Disk IDs by /dev/disk/by-id ---")
	diskIDs := make([]string, len(diskPaths))
	var errorsCollected []error

	for i, p := range diskPaths {
		diskID, err := findDiskIDByID(execute, p)
		if err != nil {
			sysutil.Error("finding ID for disk %s: %v", p, err)
			// Collect errors to report all failures at the end
			errorsCollected = append(errorsCollected, fmt.Errorf("disk '%s': %w", p, err))
			diskIDs[i] = "" // Indicate failure for this disk
		} else {
			diskIDs[i] = diskID
			sysutil.Info("Successfully found ID for disk %d (%s): %s", i+1, p, diskID)
		}
	}

	sysutil.Info("--- Disk ID Retrieval Complete ---")
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
	sysutil.Info("Wiping disk completely: %s", diskPath)

	// First try to clean ZFS-specific issues
	err := cleanZFSDisk(execute, diskPath)
	if err != nil {
		sysutil.Warn(
			"Failed to clean disk using ZFS, trying fallback options for %s: %v",
			diskPath,
			err,
		)
	}

	// Get the real device path, which handles dry run mode properly
	resolvedPath := resolveDevicePath(execute, diskPath)

	// After ZFS-specific cleanup, try the standard disk wiping methods
	// Try to wipe with sgdisk
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"sgdisk",
		"--zap-all",
		resolvedPath,
	)
	if err != nil {
		sysutil.Warn("sgdisk --zap-all failed for %s: %v", resolvedPath, err)
	}

	// Try to wipe with wipefs
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"wipefs",
		"--all",
		resolvedPath,
	)
	if err != nil {
		sysutil.Warn("wipefs failed, trying another method for %s: %v", resolvedPath, err)

		// Use dd to wipe the beginning of the disk
		_, err = sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"dd",
			"if=/dev/zero",
			"of="+resolvedPath,
			"bs=1M",
			"count=32",
			"conv=fsync",
		)
		if err != nil {
			sysutil.Warn("dd zeroing failed for %s: %v", resolvedPath, err)
		}
	}

	// Tell udev to settle down to ensure the kernel recognizes the wipe.
	if _, settleErr := sysutil.Execute(execute, sysutil.ModeNormal, "udevadm", "settle"); settleErr != nil {
		sysutil.Warn("udevadm settle after wipe failed for %s: %v", resolvedPath, settleErr)
	}

	// Use sgdisk to create a new GPT
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"sgdisk",
		"--clear",
		resolvedPath,
	)
	if err != nil {
		sysutil.Warn("sgdisk --clear failed for %s: %v", resolvedPath, err)
		return fmt.Errorf("failed to clear partition table on %s: %w", resolvedPath, err)
	}

	// Wait for changes to be recognized by the system by settling udev.
	sysutil.Info("Waiting for disk changes to be recognized...")
	if _, settleErr := sysutil.Execute(execute, sysutil.ModeNormal, "udevadm", "settle"); settleErr != nil {
		sysutil.Warn("udevadm settle after clear failed for %s: %v", resolvedPath, settleErr)
	}

	return nil
}

// resolveDevicePath resolves a possibly symlinked device path (especially from /dev/disk/by-id/)
// to its actual /dev path (e.g., /dev/sda)
func resolveDevicePath(_ bool, diskPath string) string {
	// If it's not a by-id path, just return it
	if !strings.Contains(diskPath, "/dev/disk/by-id/") {
		return diskPath
	}

	// Always try to resolve the actual device path since readlink is safe.
	// We pass 'true' for execute to ensure it always runs.
	deviceOutput, err := sysutil.Execute(
		true,
		sysutil.ModeStdOut,
		"readlink",
		"-f",
		diskPath,
	)

	if err == nil {
		resolvedPath := strings.TrimSpace(deviceOutput)
		if resolvedPath != "" {
			sysutil.Info("Resolved %s to %s", diskPath, resolvedPath)
			return resolvedPath
		}
	}

	// If readlink fails, we can log a warning.
	sysutil.Warn("could not resolve symlink %s: %v. Using original path.", diskPath, err)
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
		sysutil.Info("Checking for direct device path: %s", directPartitionPath)

		// Check multiple times for the direct path
		for attempt := 1; attempt <= 5; attempt++ {
			if _, err := os.Stat(directPartitionPath); err == nil {
				sysutil.Info("Found direct partition path: %s", directPartitionPath)
				return directPartitionPath
			}

			_, err := sysutil.Execute(execute, sysutil.ModeSilent, "partprobe", baseDiskPath)
			if err != nil {
				sysutil.Warn("partprobe failed for %s: %v", baseDiskPath, err)
			}
			sysutil.Info("Waiting for direct device path (attempt %d of 5)...", attempt)
			time.Sleep(2 * time.Second)
		}
	}

	// If direct path failed, try the by-id path
	sysutil.Info("Checking for by-id device path: %s", originalPartitionPath)
	for attempt := 1; attempt <= 5; attempt++ {
		if _, err := os.Stat(originalPartitionPath); err == nil {
			sysutil.Info("Found by-id partition path: %s", originalPartitionPath)
			return originalPartitionPath
		}

		_, err := sysutil.Execute(execute, sysutil.ModeSilent, "partprobe", diskPath)
		if err != nil {
			sysutil.Warn("partprobe failed for %s: %v", diskPath, err)
		}
		sysutil.Info("Waiting for by-id device path (attempt %d of 5)...", attempt)
		time.Sleep(2 * time.Second)
	}

	// Last resort - run partprobe globally
	sysutil.Info("Running partprobe globally to update all partition tables...")
	_, err := sysutil.Execute(execute, sysutil.ModeSilent, "partprobe")
	if err != nil {
		sysutil.Warn("partprobe failed: %v", err)
	}
	time.Sleep(3 * time.Second)

	// One last check for direct path
	if strings.HasPrefix(baseDiskPath, "/dev/") {
		baseDiskName := path.Base(baseDiskPath)
		directPartitionPath := fmt.Sprintf("/dev/%s%d", baseDiskName, partitionNumber)
		if _, err := os.Stat(directPartitionPath); err == nil {
			sysutil.Info(
				"Found direct partition path after global partprobe: %s",
				directPartitionPath,
			)
			return directPartitionPath
		}
	}

	// If we still don't have a path, warn but return empty string
	sysutil.Warn(
		"Could not find a usable device path for partition %d on %s",
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
		sysutil.Info("Waiting for partition device to be available: %s", partitionNameOriginal)

		// Try to find a usable device path
		devicePath := findPartitionDevicePath(
			execute,
			diskPath,
			partitionNumber,
			partitionNameOriginal,
		)

		// If no path was found, fall back to the original
		if devicePath == "" {
			sysutil.Warn("Using original path as fallback: %s", partitionNameOriginal)
			devicePath = partitionNameOriginal
		}

		sysutil.Info("Formatting partition using device path: %s", devicePath)

		// Prepare command arguments
		allArgs := append([]string{formatCmd}, formatArgs...)
		allArgs = append(allArgs, devicePath)

		// Execute the format command
		_, err := sysutil.Execute(
			execute,
			sysutil.ModeNormal,
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
		sysutil.Info("Would format partition: %s using command: %s", partitionNameOriginal, strings.Join(allArgs, " "))
	}

	return nil
}

//nolint:gocyclo // cleanZFSDisk properly handles ZFS disk cleaning
func cleanZFSDisk(execute bool, diskPath string) error {
	sysutil.Info("Performing thorough ZFS cleanup for disk: %s", diskPath)

	// Step 1: Force export all existing pools to disconnect them cleanly
	poolsOutput, err := sysutil.Execute(
		execute,
		sysutil.ModeStdOut,
		"zpool",
		"list",
		"-H",
		"-o",
		"name",
	)
	if err == nil && poolsOutput != "" {
		pools := strings.Split(strings.TrimSpace(poolsOutput), "\n")
		for _, pool := range pools {
			if pool == "" {
				continue
			}
			sysutil.Info("Force exporting pool: %s", pool)
			stderr, exportErr := sysutil.Execute(
				execute,
				sysutil.ModeStdErr,
				"zpool",
				"export",
				"-f",
				pool,
			)
			if exportErr != nil {
				if stderr != "" {
					sysutil.Warn("Failed to export pool %s: %s", pool, strings.TrimSpace(stderr))
				} else {
					sysutil.Warn("Failed to export pool %s: %v", pool, exportErr)
				}
			}
		}
	} else if err != nil {
		sysutil.Warn("Failed to list pools for export: %v", err)
	}

	// Step 2: Wipe ZFS labels on this specific disk using dd
	// Wipe beginning of the disk (first 1MB to cover primary label)
	const wipeSize = 2048 // sectors of 512 bytes = 1MB
	if execute {
		sysutil.Info("Wiping beginning of disk %s", diskPath)
		_, err = sysutil.Execute(
			true, // always execute dd in real mode
			sysutil.ModeNormal,
			"dd",
			"if=/dev/zero",
			"of="+diskPath,
			"bs=512",
			"count="+strconv.Itoa(wipeSize),
			"conv=fsync",
		)
		if err != nil {
			sysutil.Warn("Failed to wipe beginning of %s: %v", diskPath, err)
		}

		// Get disk size in 512-byte sectors
		sizeOutput, err := sysutil.Execute(
			true,
			sysutil.ModeStdOut,
			"blockdev",
			"--getsz",
			diskPath,
		)
		if err == nil {
			sizeStr := strings.TrimSpace(sizeOutput)
			size, parseErr := strconv.ParseInt(sizeStr, 10, 64)
			if parseErr == nil && size > wipeSize {
				seek := size - wipeSize
				sysutil.Info("Wiping end of disk %s at seek %d", diskPath, seek)
				_, err = sysutil.Execute(
					true,
					sysutil.ModeNormal,
					"dd",
					"if=/dev/zero",
					"of="+diskPath,
					"bs=512",
					"seek="+strconv.FormatInt(seek, 10),
					"count="+strconv.Itoa(wipeSize),
					"conv=fsync",
				)
				if err != nil {
					sysutil.Warn("Failed to wipe end of %s: %v", diskPath, err)
				}
			} else if parseErr != nil {
				sysutil.Warn("Failed to parse disk size '%s': %v", sizeStr, parseErr)
			}
		} else {
			sysutil.Warn("Failed to get disk size for %s: %v", diskPath, err)
		}
	} else {
		sysutil.Info("Would wipe labels on %s using dd (dry-run)", diskPath)
	}

	// Step 3: Attempt labelclear as a final cleanup
	sysutil.Info("Running labelclear on %s", diskPath)
	stderr, err := sysutil.Execute(
		execute,
		sysutil.ModeStdErr,
		"zpool",
		"labelclear",
		"-f",
		diskPath,
	)
	if err != nil {
		if stderr != "" {
			sysutil.Warn("labelclear failed for %s: %s", diskPath, strings.TrimSpace(stderr))
		} else {
			sysutil.Warn("labelclear failed for %s: %v", diskPath, err)
		}
	}

	// Step 4: Settle udev to ensure changes are recognized
	_, err = sysutil.Execute(execute, sysutil.ModeNormal, "udevadm", "settle")
	if err != nil {
		sysutil.Warn("udevadm settle failed after cleanup: %v", err)
	}

	return nil
}

// forceUnmountMountpoints unmounts all mountpoints in the /mnt/nixos directory using Linux kernel's force unmount
func forceUnmountMountpoints(execute bool) error {
	// Get all mounted filesystems from /proc/mounts
	mountsOutput, err := sysutil.Execute(
		execute,
		sysutil.ModeStdOut,
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
					sysutil.Info("Force unmounting: %s", mountpointFound)
					// Use Linux umount with force and detach options
					stderr, err := sysutil.Execute(
						execute,
						sysutil.ModeStdErr,
						"umount",
						"-f",
						"-l",
						mountpointFound,
					)
					if err != nil {
						if stderr != "" {
							sysutil.Warn(
								"Failed to force unmount %s: %s",
								mountpointFound,
								strings.TrimSpace(stderr),
							)
						} else {
							sysutil.Warn("Failed to force unmount %s: %v", mountpointFound, err)
						}
					}
				}
			}
		}
	}

	return nil
}
