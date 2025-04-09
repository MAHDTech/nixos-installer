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
// It now accepts zfsDiskIDs as a slice of strings and returns an error.
func createZFSPool(
	execute bool,
	mountPoint string,
	configData *config.Config,
	zfsDiskIDs []string,
) (zfsPoolBootName string, zfsPoolRootName string, err error) {
	log.Println("--- Creating ZFS Pools ---")

	// --- ZFS Pool Configuration ---
	zfsPoolBootName = "bpool" // Hardcoded boot pool name, consider making configurable if needed
	zfsPoolRootName = configData.ZFS.Pool.Name
	zpoolType := "" // Default to stripe/single disk
	if configData.ZFS.Pool.Mirror {
		zpoolType = "mirror"
	}
	ashift := 12 // Hardcoded ashift value, was configData.ZFS.Ashift
	zfsPoolEncryption := configData.ZFS.Pool.Encryption
	zfsDisks := configData.ZFS.Disks // Used for boot pool partition name only

	// --- ZFS Boot Pool ---
	// The boot pool traditionally uses only the first ZFS disk's boot partition.
	if len(zfsDisks) == 0 {
		log.Fatal("Cannot create ZFS boot pool: No ZFS disks specified in config.")
	}
	zfsDiskBoot := zfsDisks[0]
	partitionNameZFSBoot := fmt.Sprintf("%s1", zfsDiskBoot) // Boot partition is always 1

	log.Printf("Creating ZFS boot pool: %s on %s\n", zfsPoolBootName, partitionNameZFSBoot)
	err = utils.Execute(
		execute,
		"zpool",
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
		"-O compression=zstd", // Consider making compression configurable for boot pool
		"-O encryption=off",   // Boot pool encryption is typically not used or handled differently
		"-O version=28",       // Use ZFS version 28 for grub compatibility
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
		zfsPoolBootName,
		partitionNameZFSBoot,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create ZFS boot pool %s: %w", zfsPoolBootName, err)
	}

	// --- ZFS Root Pool ---
	log.Printf("Creating ZFS root pool: %s using type: '%s'\n", zfsPoolRootName, zpoolType)

	// Base arguments
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
		"-O compression=zstd",
		fmt.Sprintf("-O mountpoint=%s", mountPoint), // Set initial mountpoint
		fmt.Sprintf("-R %s", mountPoint),            // Set altroot
	}

	// Add encryption options if enabled
	if zfsPoolEncryption {
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
	zfsRootPoolArgs = append(zfsRootPoolArgs, zfsPoolRootName)

	// Add mirror keyword if applicable
	if zpoolType == "mirror" && len(zfsDiskIDs) > 1 {
		zfsRootPoolArgs = append(zfsRootPoolArgs, "mirror")
	}

	// Add the disk IDs (passed as []string)
	zfsRootPoolArgs = append(zfsRootPoolArgs, zfsDiskIDs...)

	// Execute the zpool create command
	err = utils.Execute(
		execute,
		"zpool",
		zfsRootPoolArgs...,
	)
	if err != nil {
		return zfsPoolBootName, "", fmt.Errorf(
			"failed to create ZFS root pool %s: %w",
			zfsPoolRootName,
			err,
		)
	}

	log.Println("--- ZFS Pool Creation Complete ---")
	return zfsPoolBootName, zfsPoolRootName, nil
}

// CreateZFSDatasets creates the necessary ZFS datasets on the root pool.
// Returns an error if any dataset creation fails.
//
//nolint:funlen
func createZFSDatasets( //nolint:gocyclo // Function complexity is high, consider refactoring later.
	execute bool,
	zfsPoolRootName string,
	mountPoint string,
	configData *config.Config,
) error {
	log.Println("--- Creating ZFS Datasets ---")

	// --- Root Dataset ---
	zfsDatasetPathRoot := path.Join(zfsPoolRootName, zfsDatasetRoot)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathRoot)
	err := utils.Execute(
		execute,
		"zfs",
		"create",
		"-o canmount=noauto", // Must be mounted manually
		"-o mountpoint=/",    // Set mountpoint within the NixOS system
		zfsDatasetPathRoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create root ZFS dataset %s: %w", zfsDatasetPathRoot, err)
	}

	// Mount the root dataset temporarily to set bootfs property
	log.Printf(
		"Temporarily mounting %s to %s for bootfs setting.\n",
		zfsDatasetPathRoot,
		mountPoint,
	)
	err = utils.Execute(
		execute,
		"mount",
		"-t zfs",
		zfsDatasetPathRoot,
		mountPoint,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to temporarily mount root dataset %s: %w",
			zfsDatasetPathRoot,
			err,
		)
	}

	// Set the bootfs property
	log.Printf("Setting bootfs property on %s to %s.\n", zfsPoolRootName, zfsDatasetPathRoot)
	err = utils.Execute(
		execute,
		"zpool",
		"set",
		fmt.Sprintf("bootfs=%s", zfsDatasetPathRoot),
		zfsPoolRootName,
	)
	if err != nil {
		// Attempt to unmount before returning the error
		errUnmount := utils.Execute(
			execute,
			"umount",
			mountPoint,
		)
		if errUnmount != nil {
			log.Printf(
				"Warning! Failed to unmount temporary root mount %s: %v\n",
				mountPoint,
				errUnmount,
			)
		}
		return fmt.Errorf("failed to set bootfs property on %s: %w", zfsPoolRootName, err)
	}

	// Unmount the root dataset
	log.Printf("Unmounting %s\n", mountPoint)
	err = utils.Execute(execute, "umount", mountPoint)
	if err != nil {
		// Log or handle unmount error? For now, just return it.
		return fmt.Errorf("failed to unmount temporary root mount %s: %w", mountPoint, err)
	}

	// --- Home Dataset ---
	zfsDatasetPathHome := path.Join(zfsPoolRootName, zfsDatasetHome)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathHome)
	err = utils.Execute(
		execute,
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
	err = utils.Execute(
		execute,
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
		pageSize, err := utils.ExecuteStdOut(
			true, // Always need page size
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

		err = utils.Execute(
			execute,
			"zfs",
			"create",
			fmt.Sprintf("-V %s", configData.Swap.Size),
			fmt.Sprintf("-b %s", pageSize),
			"-o compression=zle", // Common for swap
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
		err = utils.Execute(
			execute,
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
	err = utils.Execute(
		execute,
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
	err = utils.Execute(
		execute,
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
	err = utils.Execute(
		execute,
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
	err = utils.Execute(
		execute,
		"zfs",
		"create",
		"-o mountpoint=/var/lib/docker",
		"-o com.sun:auto-snapshot=false",
		zfsDatasetPathDocker,
	)
	if err != nil {
		return fmt.Errorf("failed to create docker ZFS dataset %s: %w", zfsDatasetPathDocker, err)
	}

	// Wait a bit for ZFS changes to settle
	if execute {
		log.Println("Waiting for ZFS changes to settle...")
		time.Sleep(5 * time.Second)
	}

	log.Println("--- ZFS Dataset Creation Complete ---")
	return nil
}
