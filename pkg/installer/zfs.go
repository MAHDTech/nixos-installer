package installer

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// CreateZFSPool creates the ZFS boot and root pools.
func createZFSPool(
	execute bool,
	mountPoint string,
	configData *config.Config,
	zfsDiskIDs []string,
) (zfsBootPoolName string, zfsRootPoolName string, err error) {
	log.Println("--- Creating ZFS Pools ---")

	// Create boot pool
	zfsBootPoolName = configData.ZFS.BootPool.Name
	err = createZFSBootPool(execute, configData)
	if err != nil {
		return "", "", fmt.Errorf("failed to create ZFS boot pool: %w", err)
	}

	// Create root pool
	zfsRootPoolName = configData.ZFS.RootPool.Name
	err = createZFSRootPool(execute, mountPoint, configData, zfsDiskIDs)
	if err != nil {
		return zfsBootPoolName, "", fmt.Errorf("failed to create ZFS root pool: %w", err)
	}

	log.Println("--- ZFS Pool Creation Complete ---")
	return zfsBootPoolName, zfsRootPoolName, nil
}

// createZFSBootPool creates the ZFS boot pool.
func createZFSBootPool(
	execute bool,
	configData *config.Config,
) error {
	zfsBootPoolName := configData.ZFS.BootPool.Name
	zfsBootPoolDisks := configData.ZFS.Disks

	if len(zfsBootPoolDisks) == 0 {
		log.Fatal("Cannot create ZFS boot pool: No ZFS disks specified in config.")
	}

	// Prepare boot partition IDs
	bootPartitions := []string{}
	for _, disk := range zfsBootPoolDisks {
		// Check if it's an NVMe disk (contains "nvme" in the path)
		if strings.Contains(disk, "nvme") {
			// NVMe disks use -partN format
			bootPartitions = append(bootPartitions, fmt.Sprintf("%s-part1", disk))
		} else {
			// Traditional SATA/SCSI disks might just append the number
			bootPartitions = append(bootPartitions, fmt.Sprintf("%s1", disk))
		}
	}

	log.Printf("Creating ZFS boot pool %s on partition %v\n", zfsBootPoolName, bootPartitions)

	// Prepare common boot pool arguments
	zfsBootPoolArgs := []string{
		"create",
		"-f",
		"-o", fmt.Sprintf("ashift=%d", configData.ZFS.Ashift),
		"-o", "autotrim=on",
		"-O", "acltype=posixacl",
		"-O", "relatime=on",
		"-O", "xattr=sa",
		"-O", "dnodesize=auto",
		"-O", "normalization=formD",
		"-O", "mountpoint=none",
		"-O", "canmount=off",
		"-O", "devices=off",
	}

	// Add compression if enabled
	if configData.ZFS.BootPool.Compression {
		zfsBootPoolArgs = append(zfsBootPoolArgs, "-O", "compression=zstd")
	} else {
		zfsBootPoolArgs = append(zfsBootPoolArgs, "-O", "compression=off")
	}

	// Boot pool specific options for bootloader compatibility
	zfsBootPoolArgs = append(
		zfsBootPoolArgs,
		"-O", "encryption=off",
		"-o", "feature@encryption=disabled",
		"-o", "feature@project_quota=disabled",
		"-o", "feature@userobj_accounting=disabled",
		"-o", "feature@bookmark_v2=disabled",
		"-o", "feature@redaction_bookmarks=disabled",
		"-o", "feature@redacted_datasets=disabled",
	)

	// Add pool name
	zfsBootPoolArgs = append(zfsBootPoolArgs, zfsBootPoolName)

	// Handle boot partition topology (mirror, stripe, or single disk)
	switch {
	case configData.ZFS.BootPool.Mirror && len(bootPartitions) > 1:
		zfsBootPoolArgs = append(zfsBootPoolArgs, "mirror")
		zfsBootPoolArgs = append(zfsBootPoolArgs, bootPartitions...)
		log.Println("Creating mirrored boot pool")
	case configData.ZFS.BootPool.Stripe && len(bootPartitions) > 1:
		// For stripe, just add all partitions (no 'stripe' keyword in zpool create)
		zfsBootPoolArgs = append(zfsBootPoolArgs, bootPartitions...)
		log.Println("Creating striped boot pool")
	default:
		// For single disk or fallback, just use the first partition
		zfsBootPoolArgs = append(zfsBootPoolArgs, bootPartitions[0])
		log.Println("Creating single-disk boot pool")
	}

	// DEBUG: Print the boot pool arguments
	log.Printf("Boot pool arguments: %v\n", zfsBootPoolArgs)

	// Execute the boot pool creation command
	_, err := utils.Execute(
		execute,
		utils.ModeNormal,
		"zpool",
		zfsBootPoolArgs...,
	)
	if err != nil {
		return fmt.Errorf("failed to create ZFS boot pool %s: %w", zfsBootPoolName, err)
	}

	return nil
}

// createZFSRootPool creates the ZFS root pool.
func createZFSRootPool(
	execute bool,
	mountPoint string,
	configData *config.Config,
	zfsDiskIDs []string,
) error {
	zfsRootPoolName := configData.ZFS.RootPool.Name

	// Prepare root partition IDs
	rootPartitions := []string{}
	for _, disk := range zfsDiskIDs {
		// Check if it's an NVMe disk (contains "nvme" in the path)
		if strings.Contains(disk, "nvme") {
			// NVMe disks use -partN format
			rootPartitions = append(rootPartitions, fmt.Sprintf("%s-part2", disk))
		} else {
			// Traditional SATA/SCSI disks might just append the number
			rootPartitions = append(rootPartitions, fmt.Sprintf("%s2", disk))
		}
	}

	log.Printf("Creating ZFS root pool %s on partition %v\n", zfsRootPoolName, rootPartitions)

	// Prepare common root pool arguments
	zfsRootPoolArgs := []string{
		"create",
		"-f",
		"-o", fmt.Sprintf("ashift=%d", configData.ZFS.Ashift),
		"-o", "autotrim=on",
		"-O", "acltype=posixacl",
		"-O", "relatime=on",
		"-O", "xattr=sa",
		"-O", "dnodesize=auto",
		"-O", "normalization=formD",
		"-O", "mountpoint=none",
		"-O", "canmount=off",
		"-O", "devices=off",
	}

	// Add compression if enabled (default is true)
	if configData.ZFS.RootPool.Compression {
		zfsRootPoolArgs = append(zfsRootPoolArgs, "-O", "compression=zstd")
	} else {
		zfsRootPoolArgs = append(zfsRootPoolArgs, "-O", "compression=off")
	}

	// Set the altroot temporary mountpoint for the install.
	zfsRootPoolArgs = append(
		zfsRootPoolArgs,
		"-R", mountPoint,
	)

	// Add encryption options if enabled
	if configData.ZFS.RootPool.Encryption {
		log.Println("ZFS Encryption is enabled for root pool.")
		zfsRootPoolArgs = append(
			zfsRootPoolArgs,
			"-O", "encryption=aes-256-gcm",
			"-O", "keylocation=prompt",
			"-O", "keyformat=passphrase",
		)
	} else {
		log.Println("ZFS Encryption is disabled for root pool.")
	}

	// Add pool name
	zfsRootPoolArgs = append(zfsRootPoolArgs, zfsRootPoolName)

	// Handle root partition topology (mirror, stripe, or single disk)
	switch {
	case configData.ZFS.RootPool.Mirror && len(rootPartitions) > 1:
		zfsRootPoolArgs = append(zfsRootPoolArgs, "mirror")
		zfsRootPoolArgs = append(zfsRootPoolArgs, rootPartitions...)
		log.Println("Creating mirrored root pool")
	case configData.ZFS.RootPool.Stripe && len(rootPartitions) > 1:
		// For stripe, just add all partitions (no 'stripe' keyword in zpool create)
		zfsRootPoolArgs = append(zfsRootPoolArgs, rootPartitions...)
		log.Println("Creating striped root pool")
	default:
		// For single disk or fallback, just use the first partition
		zfsRootPoolArgs = append(zfsRootPoolArgs, rootPartitions[0])
		log.Println("Creating single-disk root pool")
	}

	// DEBUG: Print the root pool arguments
	log.Printf("Root pool arguments: %v\n", zfsRootPoolArgs)

	// Execute the root pool creation command
	_, err := utils.Execute(
		execute,
		utils.ModeNormal,
		"zpool",
		zfsRootPoolArgs...,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to create ZFS root pool %s: %w",
			zfsRootPoolName,
			err,
		)
	}

	return nil
}

// createZFSBootDatasets creates the necessary ZFS datasets on the boot pool.
// Returns an error if any dataset creation fails.
func createZFSBootDatasets(
	execute bool,
	zfsPoolBootName string,
) error {

	var err error

	log.Printf("--- Creating ZFS Boot Datasets on pool %s ---", zfsPoolBootName)

	// --- Boot Dataset ---
	zfsDatasetPathBoot := path.Join(zfsPoolBootName, zfsDatasetBoot)

	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathBoot)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/boot",
		zfsDatasetPathBoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create boot ZFS dataset %s: %w", zfsDatasetPathBoot, err)
	}

	// Mount the boot dataset to set bootfs property
	err = mountZFSDataset(execute, zfsDatasetPathBoot)
	if err != nil {
		return fmt.Errorf("failed to mount boot filesystem: %w", err)
	}

	// Set the bootfs property on the boot pool
	log.Printf("Setting bootfs property on %s to %s.\n", zfsPoolBootName, zfsDatasetPathBoot)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zpool",
		"set",
		fmt.Sprintf("bootfs=%s", zfsDatasetPathBoot),
		zfsPoolBootName,
	)
	if err != nil {
		// Attempt to unmount before returning the error
		_, errUnmount := utils.Execute(
			execute,
			utils.ModeNormal,
			"zfs",
			"unmount",
			zfsDatasetPathBoot,
		)
		if errUnmount != nil {
			log.Printf(
				"Warning! Failed to unmount temporary root mount %s: %v\n",
				zfsDatasetPathBoot,
				errUnmount,
			)
		}
		return fmt.Errorf("failed to set bootfs property on %s: %w", zfsPoolBootName, err)
	}

	log.Println("--- ZFS Boot Dataset Creation Complete ---")
	return nil
}

// createZFSRootDatasets creates the necessary ZFS datasets on the root pool.
// Returns an error if any dataset creation fails.
func createZFSRootDatasets(
	execute bool,
	zfsPoolRootName string,
	configData *config.Config,
) error {

	var err error

	log.Printf("--- Creating ZFS Root Datasets on pool %s ---", zfsPoolRootName)

	// --- Root Dataset ---
	zfsDatasetPathRoot := path.Join(zfsPoolRootName, zfsDatasetRoot)

	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathRoot)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/",
		zfsDatasetPathRoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create root ZFS dataset %s: %w", zfsDatasetPathRoot, err)
	}

	// Mount the root dataset to set bootfs property
	err = mountZFSDataset(execute, zfsDatasetPathRoot)
	if err != nil {
		return fmt.Errorf("failed to mount root filesystem: %w", err)
	}

	// Set the bootfs property on the root pool
	log.Printf("Setting bootfs property on %s to %s.\n", zfsPoolRootName, zfsDatasetPathRoot)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zpool",
		"set",
		fmt.Sprintf("bootfs=%s", zfsDatasetPathRoot),
		zfsPoolRootName,
	)
	if err != nil {
		// Attempt to unmount before returning the error
		_, errUnmount := utils.Execute(
			execute,
			utils.ModeNormal,
			"zfs",
			"unmount",
			zfsDatasetPathRoot,
		)
		if errUnmount != nil {
			log.Printf(
				"Warning! Failed to unmount temporary root mount %s: %v\n",
				zfsDatasetPathRoot,
				errUnmount,
			)
		}
		return fmt.Errorf("failed to set bootfs property on %s: %w", zfsPoolRootName, err)
	}

	// --- Home Dataset ---
	zfsDatasetPathHome := path.Join(zfsPoolRootName, zfsDatasetHome)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathHome)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/home",
		zfsDatasetPathHome,
	)
	if err != nil {
		return fmt.Errorf("failed to create home ZFS dataset %s: %w", zfsDatasetPathHome, err)
	}

	// --- Nix Store Dataset ---
	zfsDatasetPathNix := path.Join(zfsPoolRootName, zfsDatasetNixStore)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathNix)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/nix",
		"-o", "atime=off", // Optimization for Nix store
		zfsDatasetPathNix,
	)
	if err != nil {
		return fmt.Errorf("failed to create nix ZFS dataset %s: %w", zfsDatasetPathNix, err)
	}

	// --- Swap Dataset ---
	zfsDatasetPathSwap := path.Join(zfsPoolRootName, zfsDatasetSwap)
	if configData.Swap.Enabled {
		log.Printf(
			"Creating ZFS swap volume: %s with size %s\n",
			zfsDatasetPathSwap,
			configData.Swap.Size,
		)
		// Get system page size for volblocksize
		pageSize, err := utils.Execute(
			execute,
			utils.ModeStdOut,
			"getconf",
			"PAGESIZE",
		)
		if err != nil {
			log.Printf(
				"Warning: Could not determine page size via getconf: %v. Defaulting to 16k for swap volblocksize.",
				err,
			)
			pageSize = "16384"
		} else {
			pageSize = strings.TrimSpace(pageSize)
		}

		// Make sure the page size is not empty and at least 16k
		if pageSize == "" || pageSize < "16384" {
			log.Println(
				"Warning: Could not determine page size or it was less than 16k, defaulting to 16k for swap volblocksize.",
			)
			pageSize = "16384"
		}

		// Create the swap ZFS volume
		_, err = utils.Execute(
			execute,
			utils.ModeNormal,
			"zfs",
			"create",
			"-V",
			configData.Swap.Size,
			"-b",
			pageSize,
			"-o", "compression=zle", // Different compression for swap
			"-o", "logbias=throughput",
			"-o", "sync=always",
			"-o", "primarycache=metadata",
			"-o", "secondarycache=none",
			"-o", "com.sun:auto-snapshot=false", // Don't snapshot swap
			zfsDatasetPathSwap,
		)
		if err != nil {
			return fmt.Errorf("failed to create swap ZFS volume %s: %w", zfsDatasetPathSwap, err)
		}

		// Retry logic for the swap device to appear
		if execute {
			// Wait for zvol device to appear
			swapDevicePath := fmt.Sprintf("/dev/zvol/%s", zfsDatasetPathSwap)
			log.Printf("Waiting for swap device to appear at %s", swapDevicePath)

			// Try a few times with exponential backoff
			for attempt := 1; attempt <= 5; attempt++ {
				if _, err := os.Stat(swapDevicePath); err == nil {
					log.Printf("Swap device found after %d attempts", attempt)
					break
				}

				// Trigger udev to reload devices
				_, err = utils.Execute(execute, utils.ModeNormal, "udevadm", "trigger")
				if err != nil {
					log.Printf("Warning: udevadm trigger failed: %v", err)
				}
				_, err = utils.Execute(execute, utils.ModeNormal, "udevadm", "settle")
				if err != nil {
					log.Printf("Warning: udevadm settle failed: %v", err)
				}

				// Check if the directory exists, create if needed
				dir := path.Dir(swapDevicePath)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					log.Printf("Creating directory: %s", dir)
					if err := os.MkdirAll(dir, 0750); err != nil {
						log.Printf("Warning: Failed to create directory %s: %v", dir, err)
					}
				}

				waitTime := time.Duration(attempt*2) * time.Second
				log.Printf(
					"Waiting %v seconds for swap device (attempt %d/5)...",
					waitTime.Seconds(),
					attempt,
				)
				time.Sleep(waitTime)
			}

			// Final verification
			if _, err := os.Stat(swapDevicePath); os.IsNotExist(err) {
				// Try alternative paths as fallback
				alternativePaths := []string{
					"/dev/zd0", // Sometimes used for first zvol
					fmt.Sprintf("/dev/%s/%s", zfsPoolRootName, "swap"), // Alternative path format
				}

				for _, altPath := range alternativePaths {
					log.Printf("Checking alternative swap path: %s", altPath)
					if _, err := os.Stat(altPath); err == nil {
						swapDevicePath = altPath
						log.Printf("Using alternative swap device path: %s", swapDevicePath)
						break
					}
				}
			}

			// Then format swap using the confirmed path
			log.Printf("Formatting swap volume: %s\n", swapDevicePath)
			_, err = utils.Execute(
				execute,
				utils.ModeNormal,
				"mkswap",
				swapDevicePath,
			)
			if err != nil {
				return fmt.Errorf(
					"failed to format swap volume %s: %w",
					swapDevicePath,
					err,
				)
			}
		}
	} else {
		log.Println("Skipping swap dataset creation as it is disabled.")
	}

	// --- Tmp Dataset ---
	zfsDatasetPathTmp := path.Join(zfsPoolRootName, zfsDatasetTmp)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathTmp)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/tmp",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to create tmp ZFS dataset %s: %w", zfsDatasetPathTmp, err)
	}

	// --- Var Dataset ---
	zfsDatasetPathVar := path.Join(zfsPoolRootName, zfsDatasetVar)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathVar)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var",
		zfsDatasetPathVar,
	)
	if err != nil {
		return fmt.Errorf("failed to create var ZFS dataset %s: %w", zfsDatasetPathVar, err)
	}

	// --- Var/Lib Dataset ---
	zfsDatasetPathLib := path.Join(zfsPoolRootName, zfsDatasetLib)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathLib)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var/lib",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathLib,
	)
	if err != nil {
		return fmt.Errorf("failed to create lib ZFS dataset %s: %w", zfsDatasetPathLib, err)
	}

	// --- Var/Lib/Docker Dataset ---
	zfsDatasetPathDocker := path.Join(zfsPoolRootName, zfsDatasetDocker)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathDocker)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var/lib/docker",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathDocker,
	)
	if err != nil {
		return fmt.Errorf("failed to create docker ZFS dataset %s: %w", zfsDatasetPathDocker, err)
	}

	// --- Var/Lib/Containers Dataset ---
	zfsDatasetPathContainers := path.Join(zfsPoolRootName, zfsDatasetContainers)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathContainers)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var/lib/containers",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathContainers,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to create containers ZFS dataset %s: %w",
			zfsDatasetPathContainers,
			err,
		)
	}

	// Wait a bit for ZFS changes to settle
	if execute {
		log.Println("Waiting for ZFS changes to settle...")
		time.Sleep(5 * time.Second)
	}

	log.Println("--- ZFS Dataset Creation Complete ---")
	return nil
}
