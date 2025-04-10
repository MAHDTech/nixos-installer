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

// CreateZFSPool creates the ZFS boot and root pools.
func createZFSPool(
	execute bool,
	mountPoint string,
	configData *config.Config,
	zfsDiskIDs []string,
) (zfsBootPoolName string, zfsRootPoolName string, err error) {
	log.Println("--- Creating ZFS Pools ---")

	/*
	 --- ZFS Common Pool Configuration ---
	*/
	ashift := configData.ZFS.Ashift

	// Create boot pool
	zfsBootPoolName = configData.ZFS.BootPool.Name
	zfsBootPoolDisks := configData.ZFS.Disks

	if len(zfsBootPoolDisks) == 0 {
		log.Fatal("Cannot create ZFS boot pool: No ZFS disks specified in config.")
	}

	// Prepare boot partition IDs
	bootPartitions := []string{}
	for _, disk := range zfsBootPoolDisks {
		// The boot partition is always 1
		bootPartitions = append(bootPartitions, fmt.Sprintf("%s1", disk))
	}

	log.Printf("Creating ZFS boot pool: %s\n", zfsBootPoolName)

	// Prepare common boot pool arguments
	bootPoolArgs := []string{
		"create",
		"-f",
		fmt.Sprintf("-o ashift=%d", ashift),
		"-o autotrim=on",
		"-O acltype=posixacl",
		"-O relatime=on",
		"-O xattr=sa",
		"-O dnodesize=auto",
		"-O normalization=formD",
		"-O mountpoint=none",
		"-O canmount=off",
		"-O devices=off",
	}

	// Add compression if enabled
	if configData.ZFS.BootPool.Compression {
		bootPoolArgs = append(bootPoolArgs, "-O compression=zstd")
	} else {
		bootPoolArgs = append(bootPoolArgs, "-O compression=off")
	}

	// Boot pool specific options for bootloader compatibility
	bootPoolArgs = append(bootPoolArgs,
		"-O encryption=off",
		"-O version=28",
		"-O feature@encryption=disabled",
		"-O feature@project_quota=disabled",
		"-O feature@userobj_accounting=disabled",
		"-O feature@bookmark_v2=disabled",
		"-O feature@redaction_bookmarks=disabled",
		"-O feature@redacted_datasets=disabled",
		"-O feature@bookmark_written=disabled",
		"-O feature@log_spacemap=disabled",
		"-O feature@large_dnode=disabled",
		"-O feature@sha512=disabled",
		"-O feature@skein=disabled",
		"-O feature@edonr=disabled",
		zfsBootPoolName,
	)

	// Handle pool topology (mirror, stripe, or single disk)
	if configData.ZFS.BootPool.Mirror && len(bootPartitions) > 1 {
		bootPoolArgs = append(bootPoolArgs, "mirror")
		bootPoolArgs = append(bootPoolArgs, bootPartitions...)
		log.Println("Creating mirrored boot pool")
	} else if configData.ZFS.BootPool.Stripe && len(bootPartitions) > 1 {
		// For stripe, just add all partitions (no 'stripe' keyword in zpool create)
		bootPoolArgs = append(bootPoolArgs, bootPartitions...)
		log.Println("Creating striped boot pool")
	} else {
		// For single disk or fallback, just use the first partition
		bootPoolArgs = append(bootPoolArgs, bootPartitions[0])
		log.Println("Creating single-disk boot pool")
	}

	// Execute boot pool creation command
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zpool",
		bootPoolArgs...,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create ZFS boot pool %s: %w", zfsBootPoolName, err)
	}

	/*
	 --- ZFS Root Pool Configuration ---
	*/
	zfsRootPoolName = configData.ZFS.RootPool.Name

	// Determine pool topology
	var poolTopology string
	if configData.ZFS.RootPool.Mirror && len(zfsDiskIDs) > 1 {
		poolTopology = "mirror"
	} else if configData.ZFS.RootPool.Stripe && len(zfsDiskIDs) > 1 {
		poolTopology = "stripe" // This is just for logging - zpool doesn't use "stripe" keyword
	} else {
		poolTopology = "single"
	}

	log.Printf("Creating ZFS root pool: %s using type: '%s'\n", zfsRootPoolName, poolTopology)

	// Root pool base arguments - optimized for data storage
	zfsRootPoolArgs := []string{
		"create",
		"-f",
		fmt.Sprintf("-o ashift=%d", ashift),
		"-o autotrim=on",
		"-O acltype=posixacl",
		"-O relatime=on",
		"-O xattr=sa",
		"-O dnodesize=auto",
		"-O normalization=formD",
		"-O mountpoint=none",
		"-O canmount=off",
		"-O devices=off",
	}

	// Add compression if enabled (default is true)
	if configData.ZFS.RootPool.Compression {
		zfsRootPoolArgs = append(zfsRootPoolArgs, "-O compression=zstd")
	} else {
		zfsRootPoolArgs = append(zfsRootPoolArgs, "-O compression=off")
	}

	// Add mountpoint and altroot settings
	zfsRootPoolArgs = append(zfsRootPoolArgs,
		fmt.Sprintf("-O mountpoint=%s", mountPoint),
		fmt.Sprintf("-R %s", mountPoint),
	)

	// Add encryption options if enabled
	if configData.ZFS.RootPool.Encryption {
		log.Println("ZFS Encryption is enabled for root pool.")
		zfsRootPoolArgs = append(zfsRootPoolArgs,
			"-O encryption=aes-256-gcm",
			"-O keylocation=prompt",
			"-O keyformat=passphrase",
		)
	} else {
		log.Println("ZFS Encryption is disabled for root pool.")
	}

	// Add pool name
	zfsRootPoolArgs = append(zfsRootPoolArgs, zfsRootPoolName)

	// Handle pool topology for root pool
	if poolTopology == "mirror" && len(zfsDiskIDs) > 1 {
		zfsRootPoolArgs = append(zfsRootPoolArgs, "mirror")
	}

	// Add the disk IDs (passed as []string)
	zfsRootPoolArgs = append(zfsRootPoolArgs, zfsDiskIDs...)

	// Execute the zpool create command
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zpool",
		zfsRootPoolArgs...,
	)
	if err != nil {
		return zfsBootPoolName, zfsRootPoolName, fmt.Errorf(
			"failed to create ZFS root pool %s: %w",
			zfsRootPoolName,
			err,
		)
	}

	log.Println("--- ZFS Pool Creation Complete ---")
	return zfsBootPoolName, zfsRootPoolName, nil
}

// createZFSBootDatasets creates the necessary ZFS datasets on the boot pool.
// Returns an error if any dataset creation fails.
func createZFSBootDatasets(
	execute bool,
	zfsPoolBootName string,
	mountPoint string,
	configData *config.Config,
) error {
	log.Println("--- Creating ZFS Boot Datasets on pool %s ---", zfsPoolBootName)

	// --- Boot Dataset ---
	zfsDatasetPathBoot := path.Join(zfsPoolBootName, zfsDatasetBoot)
	zfsDatasetMountPointBoot := path.Join(mountPoint, "boot")

	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathBoot)
	_, err := utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o canmount=noauto",  // Must be mounted manually
		"-o mountpoint=/boot", // Set mountpoint destination on the NixOS system
		zfsDatasetPathBoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create boot ZFS dataset %s: %w", zfsDatasetPathBoot, err)
	}

	// Mount the boot dataset temporarily to set bootfs property
	log.Printf(
		"Temporarily mounting %s to %s for bootfs setting.\n",
		zfsDatasetPathBoot,
		zfsDatasetMountPointBoot,
	)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"mount",
		"-t zfs",
		zfsDatasetPathBoot,
		zfsDatasetMountPointBoot,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to temporarily mount boot dataset %s: %w",
			zfsDatasetPathBoot,
			err,
		)
	}

	// Set the bootfs property
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
			"umount",
			zfsDatasetMountPointBoot,
		)
		if errUnmount != nil {
			log.Printf(
				"Warning! Failed to unmount temporary root mount %s: %v\n",
				zfsDatasetMountPointBoot,
				errUnmount,
			)
		}
		return fmt.Errorf("failed to set bootfs property on %s: %w", zfsPoolBootName, err)
	}

	// Unmount the root dataset
	log.Printf("Unmounting %s\n", zfsDatasetMountPointBoot)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"umount",
		zfsDatasetMountPointBoot,
	)
	if err != nil {
		// Log or handle unmount error? For now, just return it.
		return fmt.Errorf("failed to unmount temporary root mount %s: %w", mountPoint, err)
	}

	log.Println("--- ZFS Boot Dataset Creation Complete ---")
	return nil
}

// createZFSRootDatasets creates the necessary ZFS datasets on the root pool.
// Returns an error if any dataset creation fails.
func createZFSRootDatasets(
	execute bool,
	zfsPoolRootName string,
	mountPoint string,
	configData *config.Config,
) error {
	log.Println("--- Creating ZFS Root Datasets on pool %s ---", zfsPoolRootName)

	// --- Root Dataset ---
	zfsDatasetPathRoot := path.Join(zfsPoolRootName, zfsDatasetRoot)

	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathRoot)
	_, err := utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o canmount=noauto", // Must be mounted manually
		"-o mountpoint=/",    // Set mountpoint destination on the NixOS system
		zfsDatasetPathRoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create root ZFS dataset %s: %w", zfsDatasetPathRoot, err)
	}

	// --- Home Dataset ---
	zfsDatasetPathHome := path.Join(zfsPoolRootName, zfsDatasetHome)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathHome)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"create",
		"-o mountpoint=/home",
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
		"-o mountpoint=/nix",
		"-o atime=off", // Optimization for Nix store
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
				"Warning: Could not determine page size via getconf: %v. Defaulting to 4k for swap volblocksize.",
				err,
			)
			pageSize = "4k"
		} else {
			pageSize = strings.TrimSpace(pageSize)
		}

		if pageSize == "" { // Double check after trim / potential error
			log.Println(
				"Warning: Could not determine page size, defaulting to 4k for swap volblocksize.",
			)
			pageSize = "4k"
		}

		_, err = utils.Execute(
			execute,
			utils.ModeNormal,
			"zfs",
			"create",
			fmt.Sprintf("-V %s", configData.Swap.Size),
			fmt.Sprintf("-b %s", pageSize),
			"-o compression=zle", // Different compression for swap
			"-o logbias=throughput",
			"-o sync=always",
			"-o primarycache=metadata",
			"-o secondarycache=none",
			"-o com.sun:auto-snapshot=false", // Don't snapshot swap
			zfsDatasetPathSwap,
		)
		if err != nil {
			return fmt.Errorf("failed to create swap ZFS volume %s: %w", zfsDatasetPathSwap, err)
		}

		log.Printf("Formatting swap volume: /dev/zvol/%s\n", zfsDatasetPathSwap)
		_, err = utils.Execute(
			execute,
			utils.ModeNormal,
			"mkswap",
			fmt.Sprintf("/dev/zvol/%s", zfsDatasetPathSwap),
		)
		if err != nil {
			return fmt.Errorf(
				"failed to format swap volume /dev/zvol/%s: %w",
				zfsDatasetPathSwap,
				err,
			)
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
		"-o mountpoint=/tmp",
		"-o com.sun:auto-snapshot=false",
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
		"-o mountpoint=/var",
		"-o canmount=off", // Children will be mounted
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
		"-o mountpoint=/var/lib",
		"-o com.sun:auto-snapshot=false",
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
		"-o mountpoint=/var/lib/docker",
		"-o com.sun:auto-snapshot=false",
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
		"-o mountpoint=/var/lib/containers",
		zfsDatasetPathContainers,
	)
	if err != nil {
		return fmt.Errorf("failed to create containers ZFS dataset %s: %w", zfsDatasetPathContainers, err)
	}

	// Wait a bit for ZFS changes to settle
	if execute {
		log.Println("Waiting for ZFS changes to settle...")
		time.Sleep(5 * time.Second)
	}

	log.Println("--- ZFS Dataset Creation Complete ---")
	return nil
}
