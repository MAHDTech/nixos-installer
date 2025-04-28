// Package installer contains the logic for installing NixOS.
package installer

import (
	"flag"
	"fmt"
	"log"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
)

// Run function orchestrates the NixOS installation process.
// Returns an error if any step of the installation fails.
func Run() error {
	/*
	 --- Configuration and Flags ---
	*/
	configFile := flag.String(
		"config",
		"config.yaml",
		"Path to the YAML configuration file.",
	)
	execute := flag.Bool(
		"run",
		false,
		"Execute mode. (defaults to false which will run in dry-run mode.)",
	)
	executeInstall := flag.Bool(
		"install",
		false,
		"Enable to automatically install NixOS. (defaults to false which only generates the NixOS configuration)",
	)
	flag.Parse()

	if *execute {
		log.Println("Running in execute mode.")
	} else {
		log.Println("Running in dry run mode, see '-help' for more information.")
	}

	/*
	 --- Read and validate configuration ---
	*/
	configData, err := config.ReadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to read or validate configuration: %w", err)
	}

	/*
	 --- Preparation Phase ---

	 Ensure the local mountpoints are available and create the necessary directories.
	*/
	log.Println("--- Starting Preparation Phase ---")

	// Check the mountpoints.
	_, err = checkMountpoints(*execute)
	if err != nil {
		// Log non-fatal error, checking mounts is informative but not critical for proceeding
		log.Printf("Warning: Failed to check initial mountpoints: %v", err)
	}

	// Create the necessary directories.
	err = createDirectories(*execute, mountPoint, configData)
	if err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Umount all partitions on the disks.
	err = unmountDisks(*execute, configData)
	if err != nil {
		return fmt.Errorf("failed to unmount disks: %w", err)
	}

	log.Println("--- Preparation Phase Complete ---")

	/*
	 --- Disk Setup Phase ---
	*/
	log.Println("--- Starting Disk Setup Phase ---")

	// Wipe and partition the disks.
	partitionInfo, err := partitionDisks(*execute, configData)
	if err != nil {
		return fmt.Errorf("failed during disk partitioning: %w", err)
	}

	// Users might provide disks in /dev/X format.
	// We need to convert them to /dev/disk/by-id/X format as
	// ZFS pools work better with the by-id format.
	zfsDiskIDs, err := getDiskIDsByID(*execute, configData.ZFS.Disks)
	if err != nil {
		return fmt.Errorf("failed to get ZFS disk IDs: %w", err)
	}
	log.Println("--- Disk Setup Phase Complete ---")

	/*
	 --- ZFS Setup Phase ---
	*/
	log.Println("--- Starting ZFS Setup Phase ---")

	// Create the ZFS pools and capture the boot and root pool names.
	zfsPoolBootName, zfsPoolRootName, err := createZFSPool(
		*execute,
		mountPoint,
		configData,
		zfsDiskIDs,
	)
	if err != nil {
		return fmt.Errorf("failed to create ZFS pool: %w", err)
	}
	log.Printf("Created ZFS Boot Pool: %s\n", zfsPoolBootName)
	log.Printf("Created ZFS Root Pool: %s\n", zfsPoolRootName)

	// Create the ZFS datasets for the boot pool.
	err = createZFSBootDatasets(*execute, zfsPoolBootName)
	if err != nil {
		return fmt.Errorf("failed to create ZFS boot datasets on pool %s: %w", zfsPoolBootName, err)
	}

	// Create the ZFS datasets for the root pool.
	err = createZFSRootDatasets(*execute, zfsPoolRootName, configData)
	if err != nil {
		return fmt.Errorf("failed to create ZFS root datasets on pool %s: %w", zfsPoolRootName, err)
	}

	log.Println("--- ZFS Setup Phase Complete ---")

	/*
	 --- Mounting Phase ---
	*/
	log.Println("--- Starting Mounting Phase ---")
	err = mountFileSystems(
		*execute,
		mountPoint,
		configData,
		partitionInfo,
		zfsPoolBootName,
		zfsPoolRootName,
	)
	if err != nil {
		return fmt.Errorf("failed to mount filesystems: %w", err)
	}
	log.Println("--- Mounting Phase Complete ---")

	/*
	 --- NixOS Configuration Phase ---
	*/
	log.Println("--- Starting NixOS Configuration Phase ---")

	// Generate the NixOS configuration.
	err = generateNixOSConfig(*execute, mountPoint)
	if err != nil {
		return fmt.Errorf("failed to generate NixOS configuration: %w", err)
	}

	log.Println("--- NixOS Configuration Phase Complete ---")

	/*
	 --- NixOS Installation Phase ---
	*/
	log.Println("--- Starting NixOS Installation Phase ---")

	// Install NixOS.
	err = installNixOS(*execute, *executeInstall, mountPoint, configData)
	if err != nil {
		return fmt.Errorf("failed during NixOS installation: %w", err)
	}

	log.Println("--- NixOS Installation Phase Complete ---")

	log.Println("### Finished NixOS installation process ###")
	return nil
}
