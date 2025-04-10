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

// PartitionInfo holds information about created partitions
type PartitionInfo struct {
	UEFI        string   // UEFI/ESP partition
	NixOSConfig string   // NixOS configuration partition
	ZFSBoot     []string // ZFS boot partitions can be one or more
	ZFSData     []string // ZFS data/root partitions can be one or more
}

// PartitionDisks handles partitioning the specified disks according to the configuration.
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
		return PartitionInfo{}, fmt.Errorf("failed to partition UEFI disk %s: %w", uefiDiskConfig.Disk, err)
	}

	/*
	 --- NixOS Config Disk ---

	 (if enabled, uses same disk as UEFI)
	*/
	nixosConfigDisk := configData.UEFI.Disk
	if configData.NixOS.Config.Enabled {
		partInfo.NixOSConfig, err = partitionNixOSConfigDisk(execute, nixosConfigDisk, uefiDiskConfig.Disk)
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

// PartitionUEFIDisk handles partitioning and formatting for the UEFI disk.
// Returns the UEFI partition name or an error.
func partitionUEFIDisk(execute bool, configData *config.Config) (partitionNameUEFI string, err error) {
	partitionNumberUEFI := 1
	partitionNameUEFI = fmt.Sprintf("%s%d", configData.UEFI.Disk, partitionNumberUEFI)
	partitionSizeUEFI := configData.UEFI.Size

	log.Printf("Partitioning UEFI disk: %s\n", configData.UEFI.Disk)

	// Delete existing partitions.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--zap-all=%s", configData.UEFI.Disk),
	)
	if err != nil {
		return "", fmt.Errorf("sgdisk --zap-all failed for %s: %w", configData.UEFI.Disk, err)
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
		return "", fmt.Errorf("sgdisk --new (UEFI partition) failed for %s: %w", configData.UEFI.Disk, err)
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
		log.Printf("Warning: sgdisk --print failed for %s after partitioning: %v", configData.UEFI.Disk, err)
	}

	log.Printf("Formatting UEFI partition: %s\n", partitionNameUEFI)

	// Format the UEFI partition.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// If the NixOS config disk is the same as the UEFI disk, use the next partition number
	var partitionNumberNixOSConfig int
	if nixosConfigDisk == uefiDisk {
		partitionNumberNixOSConfig = 2 // UEFI is partition 1
	} else {
		partitionNumberNixOSConfig = 1
	}
	partitionNameNixOSConfig = fmt.Sprintf("%s%d", nixosConfigDisk, partitionNumberNixOSConfig)

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

	// Format the NixOS config partition.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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
	configData *config.Config,
) (partitionNameZFSBoot string, partitionNameZFSData string, err error) {
	partitionNumberZFSBoot := 1
	partitionNumberZFSData := 2
	partitionNameZFSBoot = fmt.Sprintf("%s%d", zfsDisk, partitionNumberZFSBoot)
	partitionNameZFSData = fmt.Sprintf("%s%d", zfsDisk, partitionNumberZFSData)

	log.Printf("Partitioning ZFS disk %d: %s\n", index+1, zfsDisk)

	// Delete all existing partitions.
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"sgdisk",
		fmt.Sprintf("--zap-all=%s", zfsDisk),
	)
	if err != nil {
		return "", "", fmt.Errorf("sgdisk --zap-all failed for ZFS disk %s: %w", zfsDisk, err)
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

// GetZFSDiskIDs finds the /dev/disk/by-id/ paths for the ZFS data partitions.
// Returns a slice of disk IDs or an error if any ID cannot be found.
func getZFSDiskIDs(execute bool, zfsDisks []string) (zfsDiskIDs []string, err error) {

	log.Println("--- Retrieving ZFS Disk IDs ---")

	zfsDiskIDs = make([]string, len(zfsDisks))

	// If in dry run mode, generate simulated IDs
	if !execute {
		for index, zfsDisk := range zfsDisks {
			// Create a simulated ID for dry run
			partitionSuffix := fmt.Sprintf("%s2", path.Base(zfsDisk))
			simID := fmt.Sprintf("/dev/disk/by-id/%s-part2-simulated", partitionSuffix)
			zfsDiskIDs[index] = simID
			log.Printf("Dry run: Simulating ZFS data disk %d ID: %s\n", index+1, simID)
		}
		return zfsDiskIDs, nil
	}

	// Get the real IDs for the ZFS data partitions when not in dry-run mode.
	for index, zfsDisk := range zfsDisks {
		partitionSuffix := fmt.Sprintf("%s2", path.Base(zfsDisk))

		// Sometimes /dev/disk/by-id takes a moment to update
		// so we need to retry a few times.
		var diskID string
		var execErr error
		for attempt := 0; attempt < 5; attempt++ {

			diskID, execErr = utils.Execute(
				execute,
				utils.ModeStdOut,
				"find",
				"/dev/disk/by-id/",
				"-lname",
				fmt.Sprintf("*%s", partitionSuffix),
			)
			if execErr != nil {
				log.Printf(
					"Warning: 'find /dev/disk/by-id' command failed for %s (attempt %d): %v",
					partitionSuffix,
					attempt+1,
					execErr,
				)
				// Don't break here, maybe the command works next time or the ID appears
			}

			diskID = strings.TrimSpace(diskID)
			if diskID != "" {
				break // Found the ID
			}

			// Wait regardless of execute flag because the ID is needed even for dry run.
			log.Printf("Waiting for /dev/disk/by-id for %s (attempt %d)...", zfsDisk, attempt+1)
			time.Sleep(1 * time.Second)

		}

		if diskID == "" {
			// If execErr was nil but diskID is still empty, the partition wasn't found
			return nil, fmt.Errorf(
				"could not find /dev/disk/by-id/ for ZFS data partition %s2 after multiple attempts",
				zfsDisk,
			)
		}

		zfsDiskIDs[index] = diskID
		log.Printf("Found ZFS data disk %d ID: %s for partition %s\n", index+1, diskID, partitionSuffix)
	}

	log.Println("--- ZFS Disk ID Retrieval Complete ---")
	return zfsDiskIDs, nil
}
