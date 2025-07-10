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
	partitionInfo, err := wipeAndPartitionDisks(*execute, configData)
	if err != nil {
		return fmt.Errorf("failed during disk partitioning: %w", err)
	}

	// Users might provide disks in /dev/X format.
	// We need to convert them to /dev/disk/by-id/X format as
	// ZFS pools work better with the by-id format.

	// Get the disk IDs for the pool.
	zfsDiskIDsPoolCache, err := getDiskIDsByID(*execute, configData.ZFS.Pool.Disks.Cache)
	if err != nil {
		return fmt.Errorf("failed to get ZFS disk IDs for the pool cache: %w", err)
	}
	zfsDiskIDsPoolLog, err := getDiskIDsByID(*execute, configData.ZFS.Pool.Disks.Log)
	if err != nil {
		return fmt.Errorf("failed to get ZFS disk IDs for the pool log: %w", err)
	}
	zfsDiskIDsPoolData, err := getDiskIDsByID(*execute, configData.ZFS.Pool.Disks.Data)
	if err != nil {
		return fmt.Errorf("failed to get ZFS disk IDs for the pool data: %w", err)
	}
	zfsDiskIDsPoolSpare, err := getDiskIDsByID(*execute, configData.ZFS.Pool.Disks.Spare)
	if err != nil {
		return fmt.Errorf("failed to get ZFS disk IDs for the pool spare: %w", err)
	}

	log.Println("--- Disk Setup Phase Complete ---")

	/*
	 --- ZFS Setup Phase ---
	*/
	log.Println("--- Starting ZFS Setup Phase ---")

	// Create the ZFS pool.
	err = createZFSPool(
		*execute,
		mountPoint,
		configData.ZFS.Pool.Name,
		configData.ZFS.Pool.Compression,
		configData.ZFS.Pool.Type,
		zfsDiskIDsPoolCache,
		zfsDiskIDsPoolLog,
		zfsDiskIDsPoolData,
		zfsDiskIDsPoolSpare,
		configData.ZFS.Ashift,
	)
	if err != nil {
		return fmt.Errorf("failed to create ZFS pool: %w", err)
	}
	log.Printf("Created ZFS Pool: %s\n", configData.ZFS.Pool.Name)

	// Create the ZFS datasets for the pool.
	err = createZFSDatasets(*execute, configData.ZFS.Pool.Name, configData)
	if err != nil {
		return fmt.Errorf(
			"failed to create ZFS datasets on pool %s: %w",
			configData.ZFS.Pool.Name,
			err,
		)
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
		configData.ZFS.Pool.Name,
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
