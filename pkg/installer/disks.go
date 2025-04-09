package installer

import (
	"fmt"
	"log"
	"path"
	"strings"
	"time"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// PartitionDisks handles partitioning the specified disks according to the configuration.
// Returns partition names and an error if any partitioning step fails.
func partitionDisks(
	execute bool,
	configData *config.Config,
) (string, string, string, string, error) {
	log.Println("--- Partitioning Disks ---")

	// --- UEFI Disk ---
	uefiDisk := configData.UEFI.Disk
	partitionNameUEFI, err := partitionUEFIDisk(execute, uefiDisk)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to partition UEFI disk %s: %w", uefiDisk, err)
	}

	// --- NixOS Config Disk (if enabled, uses same disk as UEFI) ---
	nixosConfigDisk := configData.UEFI.Disk // Use UEFI disk for NixOS config partition
	partitionNameNixOSConfig := ""          // Initialize
	if configData.NixOS.Config.Enabled {
		partitionNameNixOSConfig, err = partitionNixOSConfigDisk(execute, nixosConfigDisk, uefiDisk)
		if err != nil {
			return "", "", "", "", fmt.Errorf(
				"failed to partition NixOS config on disk %s: %w",
				nixosConfigDisk,
				err,
			)
		}
	}

	// --- ZFS Disks ---
	zfsDisks := configData.ZFS.Disks
	var partitionNameZFSBoot string
	var partitionNameZFSData string
	if len(zfsDisks) > 0 {
		bootPart, dataPart, err := partitionZFSDisk(execute, zfsDisks[0], 0)
		if err != nil {
			return "", "", "", "", fmt.Errorf(
				"failed to partition ZFS disk %s: %w",
				zfsDisks[0],
				err,
			)
		}
		partitionNameZFSBoot = bootPart
		partitionNameZFSData = dataPart
	}

	// Sleep briefly to allow the kernel to recognize new partitions
	if execute {
		log.Println("Waiting 5 seconds for partitions to settle...")
		time.Sleep(5 * time.Second)
	}

	log.Println("--- Disk Partitioning Complete ---")
	return partitionNameUEFI, partitionNameNixOSConfig, partitionNameZFSBoot, partitionNameZFSData, nil
}

// PartitionUEFIDisk handles partitioning and formatting for the UEFI disk.
// Returns the UEFI partition name or an error.
func partitionUEFIDisk(execute bool, uefiDisk string) (partitionNameUEFI string, err error) {
	partitionNumberUEFI := 1
	partitionNameUEFI = fmt.Sprintf("%s%d", uefiDisk, partitionNumberUEFI)

	log.Printf("Partitioning UEFI disk: %s\n", uefiDisk)
	err = utils.Execute(
		execute,
		"sgdisk",
		fmt.Sprintf("--zap-all=%s", uefiDisk),
	)
	if err != nil {
		return "", fmt.Errorf("sgdisk --zap-all failed for %s: %w", uefiDisk, err)
	}
	err = utils.Execute(
		execute,
		"sgdisk",
		fmt.Sprintf("--new=%d:1M:1G", partitionNumberUEFI),
		fmt.Sprintf("--typecode=%d:ef00", partitionNumberUEFI),
		fmt.Sprintf("--change-name=%d:uefi", partitionNumberUEFI),
		uefiDisk,
	)
	if err != nil {
		return "", fmt.Errorf("sgdisk --new (UEFI partition) failed for %s: %w", uefiDisk, err)
	}
	err = utils.Execute(
		execute,
		"sgdisk",
		fmt.Sprintf("--print=%s", uefiDisk),
	)
	if err != nil {
		// Log print error but don't fail the whole operation
		log.Printf("Warning: sgdisk --print failed for %s after partitioning: %v", uefiDisk, err)
	}
	log.Printf("Formatting UEFI partition: %s\n", partitionNameUEFI)
	err = utils.Execute(
		execute,
		"mkfs.fat",
		"-F",
		"32",
		partitionNameUEFI,
	)
	if err != nil {
		return "", fmt.Errorf("mkfs.fat failed for %s: %w", partitionNameUEFI, err)
	}

	return partitionNameUEFI, nil
}

// PartitionNixOSConfigDisk handles partitioning and formatting for the NixOS config disk.
// It assumes the NixOS config partition lives on the same physical disk as the UEFI partition.
// Returns the NixOS config partition name or an error.
func partitionNixOSConfigDisk(
	execute bool,
	nixosConfigDisk string,
	uefiDisk string,
) (partitionNameNixOSConfig string, err error) {
	partitionNumberNixOSConfig := 1
	// If the NixOS config disk is the same as the UEFI disk, use the next partition number
	if nixosConfigDisk == uefiDisk {
		partitionNumberNixOSConfig = 2 // UEFI is partition 1
	}
	partitionNameNixOSConfig = fmt.Sprintf("%s%d", nixosConfigDisk, partitionNumberNixOSConfig)

	log.Printf(
		"Partitioning NixOS configuration on disk: %s using partition %d.\n",
		nixosConfigDisk,
		partitionNumberNixOSConfig,
	)

	err = utils.Execute(
		execute,
		"sgdisk",
		fmt.Sprintf("--new=%d:0:0", partitionNumberNixOSConfig),
		fmt.Sprintf("--typecode=%d:8300", partitionNumberNixOSConfig), // Linux filesystem
		fmt.Sprintf("--change-name=%d:nixos-config", partitionNumberNixOSConfig),
		nixosConfigDisk,
	)
	if err != nil {
		return "", fmt.Errorf(
			"sgdisk --new (NixOS config partition) failed for %s: %w",
			nixosConfigDisk,
			err,
		)
	}
	err = utils.Execute(
		execute,
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
	err = utils.Execute(
		execute,
		"mkfs.xfs", // Using XFS as specified in mount logic
		"-L",
		"nixos-config",
		partitionNameNixOSConfig,
	)
	if err != nil {
		return "", fmt.Errorf("mkfs.xfs failed for %s: %w", partitionNameNixOSConfig, err)
	}

	return partitionNameNixOSConfig, nil
}

// PartitionZFSDisk handles partitioning for a single ZFS disk.
// Returns ZFS boot and data partition names or an error.
func partitionZFSDisk(
	execute bool,
	zfsDisk string,
	index int,
) (partitionNameZFSBoot string, partitionNameZFSData string, err error) {
	partitionNumberZFSBoot := 1
	partitionNumberZFSData := 2
	partitionNameZFSBoot = fmt.Sprintf("%s%d", zfsDisk, partitionNumberZFSBoot)
	partitionNameZFSData = fmt.Sprintf("%s%d", zfsDisk, partitionNumberZFSData)

	log.Printf("Partitioning ZFS disk %d: %s\n", index+1, zfsDisk)

	// Delete existing partitions.
	err = utils.Execute(
		execute,
		"sgdisk",
		fmt.Sprintf("--zap-all=%s", zfsDisk),
	)
	if err != nil {
		return "", "", fmt.Errorf("sgdisk --zap-all failed for ZFS disk %s: %w", zfsDisk, err)
	}
	err = utils.Execute(
		execute,
		"sgdisk",
		// Hardcode boot partition size to 1G as it's not in config.
		// This mirrors the original implicit UEFI partition size.
		fmt.Sprintf(
			"--new=%d:1M:%s",
			partitionNumberZFSBoot,
			"1G",
		), // Was configData.ZFS.BootSize
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
	err = utils.Execute(
		execute,
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
	err = utils.Execute(
		execute,
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

// GetZFSDiskIDs finds the /dev/disk/by-id/ paths for the ZFS data partitions.
// Returns a slice of disk IDs or an error if any ID cannot be found.
func getZFSDiskIDs(zfsDisks []string) (zfsDiskIDs []string, err error) {
	log.Println("--- Retrieving ZFS Disk IDs ---")
	zfsDiskIDs = make([]string, len(zfsDisks))
	for i, zfsDisk := range zfsDisks {
		partitionSuffix := fmt.Sprintf("%s2", path.Base(zfsDisk))

		// Sometimes /dev/disk/by-id takes a moment to update
		var diskID string
		var execErr error
		for j := 0; j < 5; j++ { // Retry a few times
			diskID, execErr = utils.ExecuteStdOut(
				true, // Always execute find, even in dry-run, as we need the ID
				"find",
				"/dev/disk/by-id/",
				"-lname",
				fmt.Sprintf("*%s", partitionSuffix),
			)
			if execErr != nil {
				log.Printf(
					"Warning: 'find /dev/disk/by-id' command failed for %s (attempt %d): %v",
					partitionSuffix,
					j+1,
					execErr,
				)
				// Don't break here, maybe the command works next time or the ID appears
			}
			diskID = strings.TrimSpace(diskID)
			if diskID != "" {
				break // Found the ID
			}
			// Wait regardless of execute flag because the ID is needed even for dry run.
			log.Printf("Waiting for /dev/disk/by-id for %s (attempt %d)...", zfsDisk, j+1)
			time.Sleep(1 * time.Second)
		}
		if diskID == "" {
			// If execErr was nil but diskID is still empty, the partition wasn't found
			return nil, fmt.Errorf(
				"could not find /dev/disk/by-id/ for ZFS data partition %s2 after multiple attempts",
				zfsDisk,
			)
		}

		zfsDiskIDs[i] = diskID
		log.Printf("Found ZFS data disk %d ID: %s for partition %s\n", i+1, diskID, partitionSuffix)
	}
	log.Println("--- ZFS Disk ID Retrieval Complete ---")
	return zfsDiskIDs, nil
}
